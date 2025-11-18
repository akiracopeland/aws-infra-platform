package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

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
}

func (d *ServerDeps) CreateDeployment(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateDeploymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	// 3) Insert into runs (action=plan, status=queued)
	res, err = tx.ExecContext(ctx, `
		INSERT INTO runs (
			deployment_id,
			action,
			status,
			artifacts_uri,
			summary,
			started_at,
			finished_at
		) VALUES (?, 'plan', 'queued', NULL, NULL, NULL, NULL)
	`,
		deploymentID,
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
		"action":        "plan",
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
