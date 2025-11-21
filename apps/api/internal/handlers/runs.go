package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GET /v1/runs/:id
func (d *ServerDeps) GetRun(c *gin.Context) {
	idStr := c.Param("id")
	runID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || runID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
		return
	}

	var (
		deploymentID int64
		action       string
		status       string
		summary      sql.NullString
		startedAt    sql.NullTime
		finishedAt   sql.NullTime
	)

	err = d.DB.QueryRowContext(
		c.Request.Context(),
		`SELECT deployment_id, action, status, summary, started_at, finished_at
         FROM runs
         WHERE id = ?`,
		runID,
	).Scan(&deploymentID, &action, &status, &summary, &startedAt, &finishedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query run"})
		return
	}

	resp := gin.H{
		"id":           runID,
		"deploymentId": deploymentID,
		"action":       action,
		"status":       status,
	}

	if summary.Valid {
		resp["summary"] = summary.String
	}
	if startedAt.Valid {
		resp["startedAt"] = startedAt.Time
	}
	if finishedAt.Valid {
		resp["finishedAt"] = finishedAt.Time
	}

	c.JSON(http.StatusOK, resp)
}
