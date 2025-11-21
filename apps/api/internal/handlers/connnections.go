package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gin-gonic/gin"
)

type CreateAWSConnectionReq struct {
	RoleArn    string `json:"roleArn" binding:"required"`
	ExternalID string `json:"externalId" binding:"required"`
	Region     string `json:"region" binding:"required"`
	Nickname   string `json:"nickname"`
}

type AWSConnectionResp struct {
	ID        int64  `json:"id"`
	OrgID     int64  `json:"orgId"`
	AccountID string `json:"accountId"`
	RoleArn   string `json:"roleArn"`
	Region    string `json:"region"`
	Nickname  string `json:"nickname,omitempty"`
}

var roleArnRE = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):role\/.+$`)

// POST /v1/connections/aws
func (d *ServerDeps) CreateAWSConnection(c *gin.Context) {
	var req CreateAWSConnectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m := roleArnRE.FindStringSubmatch(req.RoleArn)
	if m == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid roleArn format"})
		return
	}
	accountID := m[1]

	// Hard-coded for now; later: derive from JWT / org context.
	const orgID int64 = 1
	const userID int64 = 1

	// 1) Validate role via STS AssumeRole
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(req.Region))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load AWS config"})
		return
	}

	stsClient := sts.NewFromConfig(awsCfg)
	_, err = stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         &req.RoleArn,
		RoleSessionName: awsString("aip-validate"),
		ExternalId:      awsString(req.ExternalID),
		DurationSeconds: awsInt32(900),
	})
	if err != nil {
		// Surface as 400 so the user can fix the role/externalId.
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to assume role: " + err.Error()})
		return
	}

	// 2) Insert into aws_connections
	res, err := d.DB.ExecContext(ctx, `
		INSERT INTO aws_connections (
			org_id,
			account_id,
			role_arn,
			external_id,
			region,
			nickname,
			created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, orgID, accountID, req.RoleArn, req.ExternalID, req.Region, req.Nickname, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert aws_connection: " + err.Error()})
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get connection id"})
		return
	}

	resp := AWSConnectionResp{
		ID:        id,
		OrgID:     orgID,
		AccountID: accountID,
		RoleArn:   req.RoleArn,
		Region:    req.Region,
		Nickname:  req.Nickname,
	}

	c.JSON(http.StatusCreated, resp)
}

// GET /v1/connections/aws
func (d *ServerDeps) ListAWSConnections(c *gin.Context) {
	// For now we hardcode org_id = 1 (same as POST)
	const orgID int64 = 1

	ctx := c.Request.Context()

	rows, err := d.DB.QueryContext(ctx, `
        SELECT id, org_id, account_id, role_arn, region, nickname
        FROM aws_connections
        WHERE org_id = ?
        ORDER BY created_at DESC
    `, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query aws_connections"})
		return
	}
	defer rows.Close()

	var conns []AWSConnectionResp

	for rows.Next() {
		var conn AWSConnectionResp
		var nickname sql.NullString

		if err := rows.Scan(
			&conn.ID,
			&conn.OrgID,
			&conn.AccountID,
			&conn.RoleArn,
			&conn.Region,
			&nickname,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan aws_connection"})
			return
		}

		if nickname.Valid {
			conn.Nickname = nickname.String
		}

		conns = append(conns, conn)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rows error"})
		return
	}

	c.JSON(http.StatusOK, conns)
}

// small helpers (same as in main.go, duplicated here for now)
func awsString(s string) *string { return &s }
func awsInt32(i int32) *int32    { return &i }
