package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

type DeploymentSummary struct {
	ID                int64      `json:"id"`
	BlueprintKey      string     `json:"blueprintKey"`
	Version           string     `json:"version"`
	EnvironmentID     int64      `json:"environmentId"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	LastRunID         *int64     `json:"lastRunId,omitempty"`
	LastRunStatus     *string    `json:"lastRunStatus,omitempty"`
	LastRunStartedAt  *time.Time `json:"lastRunStartedAt,omitempty"`
	LastRunFinishedAt *time.Time `json:"lastRunFinishedAt,omitempty"`

	// Raw terraform outputs JSON (terraform output -json)
	OutputsJSON json.RawMessage `json:"outputsJson,omitempty"`
}

// Dependencies for handlers
type ServerDeps struct {
	DB  *sql.DB
	RDB *redis.Client
}

// Request body for creating a deployment
type CreateDeploymentReq struct {
	BlueprintKey  string         `json:"blueprintKey" binding:"required"`
	Version       string         `json:"version" binding:"required"`
	EnvironmentID int64          `json:"environmentId" binding:"required"`
	Inputs        map[string]any `json:"inputs"`
	AWS           struct {
		RoleArn    string `json:"roleArn" binding:"required"`
		ExternalID string `json:"externalId" binding:"required"`
		Region     string `json:"region" binding:"required"`
	} `json:"aws"`
	Action string `json:"action"` // "plan" or "apply" (optional, defaults to "plan")
}

func (d *ServerDeps) CreateDeployment(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateDeploymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalize & validate action
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "plan"
	}
	if action != "plan" && action != "apply" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action; must be 'plan' or 'apply'"})
		return
	}

	// Temporary: we use the seeded user ID = 1.
	const hardcodedUserID int64 = 1

	// Convert inputs to JSON for deployments.inputs_json
	inputsJSON, err := json.Marshal(req.Inputs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to encode inputs"})
		return
	}

	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback() // safe even if we commit

	// 1) Look up blueprint_id from key + version
	var blueprintID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM blueprints WHERE blueprint_key = ? AND version = ?`,
		req.BlueprintKey, req.Version,
	).Scan(&blueprintID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown blueprint"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve blueprint: " + err.Error()})
		return
	}

	// 2) Insert into deployments
	res, err := tx.ExecContext(ctx, `
		INSERT INTO deployments (
			blueprint_id,
			environment_id,
			status,
			inputs_json,
			created_by
		) VALUES (?, ?, 'pending', ?, ?)
	`,
		blueprintID,
		req.EnvironmentID,
		string(inputsJSON),
		hardcodedUserID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert deployment: " + err.Error()})
		return
	}

	deploymentID, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get deployment id"})
		return
	}

	// 3) Insert into runs (action from request, status=queued)
	res, err = tx.ExecContext(ctx, `
		INSERT INTO runs (
			deployment_id,
			action,
			status,
			artifacts_uri,
			summary,
			started_at,
			finished_at
		) VALUES (?, ?, 'queued', NULL, NULL, NULL, NULL)
	`,
		deploymentID,
		action,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert run: " + err.Error()})
		return
	}

	runID, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get run id"})
		return
	}

	// 4) Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit transaction"})
		return
	}

	// 5) Enqueue job with the real runID
	job := map[string]any{
		"run_id":        runID,
		"action":        action,
		"blueprint_key": req.BlueprintKey,
		"version":       req.Version,
		"inputs":        req.Inputs,
		"aws": map[string]string{
			"roleArn":    req.AWS.RoleArn,
			"externalId": req.AWS.ExternalID,
			"region":     req.AWS.Region,
		},
	}

	payload, err := json.Marshal(job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode job"})
		return
	}

	if err := d.RDB.RPush(context.Background(), "aip:jobs", payload).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue job: " + err.Error()})
		return
	}

	// 6) Return real IDs
	c.JSON(http.StatusAccepted, gin.H{
		"deploymentId": deploymentID,
		"runId":        runID,
		"status":       "queued",
	})
}

// GET /v1/deployments
// For now: environment_id is hard-coded to 1 (dev)
func (d *ServerDeps) ListDeployments(c *gin.Context) {
	const envID int64 = 1

	ctx := c.Request.Context()

	rows, err := d.DB.QueryContext(ctx, `
        SELECT
            dep.id,
            bp.blueprint_key,
            bp.version,
            dep.environment_id,
            dep.status,
            dep.created_at,
            dep.outputs_json,
            lr.id AS last_run_id,
            lr.status AS last_run_status,
            lr.started_at AS last_run_started_at,
            lr.finished_at AS last_run_finished_at
        FROM deployments dep
        JOIN blueprints bp ON dep.blueprint_id = bp.id
        LEFT JOIN (
            SELECT r1.*
            FROM runs r1
            JOIN (
                SELECT deployment_id, MAX(id) AS max_id
                FROM runs
                GROUP BY deployment_id
            ) mr
            ON mr.deployment_id = r1.deployment_id AND mr.max_id = r1.id
        ) lr
        ON lr.deployment_id = dep.id
        WHERE dep.environment_id = ?
        ORDER BY dep.created_at DESC
    `, envID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query deployments"})
		return
	}
	defer rows.Close()

	var list []DeploymentSummary

	for rows.Next() {
		var s DeploymentSummary
		var lastRunID sql.NullInt64
		var lastRunStatus sql.NullString
		var lastRunStartedAt sql.NullTime
		var lastRunFinishedAt sql.NullTime
		var outputsRaw sql.NullString

		if err := rows.Scan(
			&s.ID,
			&s.BlueprintKey,
			&s.Version,
			&s.EnvironmentID,
			&s.Status,
			&s.CreatedAt,
			&outputsRaw,
			&lastRunID,
			&lastRunStatus,
			&lastRunStartedAt,
			&lastRunFinishedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan deployments"})
			return
		}

		if lastRunID.Valid {
			id := lastRunID.Int64
			s.LastRunID = &id
		}
		if lastRunStatus.Valid {
			status := lastRunStatus.String
			s.LastRunStatus = &status
		}
		if lastRunStartedAt.Valid {
			t := lastRunStartedAt.Time
			s.LastRunStartedAt = &t
		}
		if lastRunFinishedAt.Valid {
			t := lastRunFinishedAt.Time
			s.LastRunFinishedAt = &t
		}
		if outputsRaw.Valid {
			s.OutputsJSON = json.RawMessage(outputsRaw.String)
		}

		list = append(list, s)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rows error"})
		return
	}

	c.JSON(http.StatusOK, list)
}
