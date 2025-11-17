package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

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

// POST /v1/deployments
// For now: validate payload and push a job to Redis
func (d *ServerDeps) CreateDeployment(c *gin.Context) {
	var req CreateDeploymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: insert into deployments & runs tables and use real IDs.
	// For now, use a timestamp as a fake run ID.
	runID := time.Now().Unix()

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

	c.JSON(http.StatusAccepted, gin.H{
		"runId":  runID,
		"status": "queued",
	})
}
