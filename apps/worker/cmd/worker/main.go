package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	_ "github.com/go-sql-driver/mysql"
	redis "github.com/redis/go-redis/v9"
)

type Job struct {
	RunID        int64             `json:"run_id"`
	Action       string            `json:"action"`
	BlueprintKey string            `json:"blueprint_key"`
	Version      string            `json:"version"`
	Inputs       map[string]any    `json:"inputs"`
	AWS          map[string]string `json:"aws"` // expects roleArn, externalId, region
}

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	// DB for updating runs table
	dsn := getEnv("MYSQL_DSN", "aip:aip@tcp(127.0.0.1:3306)/aws_infra_platform?parseTime=true&multiStatements=true")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	log.Printf("worker listening on queue aip:jobs (redis=%s)", redisAddr)

	for {
		res, err := rdb.BLPop(ctx, 5*time.Second, "aip:jobs").Result()
		if err == redis.Nil {
			continue // timeout, just loop again
		}
		if err != nil {
			log.Printf("BLPop error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if len(res) != 2 {
			continue
		}

		var job Job
		if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
			log.Printf("decode job error: %v", err)
			continue
		}

		log.Printf("job: run=%d action=%s blueprint=%s@%s", job.RunID, job.Action, job.BlueprintKey, job.Version)

		// Mark run as running
		if err := markRunRunning(ctx, db, job.RunID); err != nil {
			log.Printf("failed to mark run %d running: %v", job.RunID, err)
		}

		if err := handleJob(ctx, db, &job); err != nil {
			log.Printf("job %d failed: %v", job.RunID, err)
			if err2 := markRunFailed(ctx, db, job.RunID, err.Error()); err2 != nil {
				log.Printf("failed to mark run %d failed: %v", job.RunID, err2)
			}
		} else {
			log.Printf("job %d completed successfully", job.RunID)
			if err := markRunSucceeded(ctx, db, job.RunID); err != nil {
				log.Printf("failed to mark run %d succeeded: %v", job.RunID, err)
			}
		}
	}
}

// handleJob now has access to the DB so we can persist outputs on apply
func handleJob(ctx context.Context, db *sql.DB, job *Job) error {
	switch job.Action {
	case "plan":
		return runTerraformPlan(ctx, job)

	case "apply":
		// 1) Run apply
		if err := runTerraformApply(ctx, job); err != nil {
			return err
		}

		// 2) Best-effort: capture terraform outputs
		outputsJSON, err := captureTerraformOutputs(ctx, job)
		if err != nil {
			log.Printf("run %d: terraform apply succeeded but failed to capture outputs: %v", job.RunID, err)
			// Do not fail the run just because outputs capture failed
			return nil
		}

		// 3) Persist outputs into deployments.outputs_json via the run → deployment relation
		if err := persistOutputsForRun(ctx, db, job.RunID, outputsJSON); err != nil {
			log.Printf("run %d: failed to persist outputs: %v", job.RunID, err)
		}

		return nil

	default:
		log.Printf("unsupported action %q, skipping", job.Action)
		return nil
	}
}

func runTerraformPlan(ctx context.Context, job *Job) error {
	modulePath, err := modulePathFor(job.BlueprintKey)
	if err != nil {
		return err
	}

	// Assume role for this job (using values from job.AWS)
	creds, err := assumeRoleForJob(ctx, job)
	if err != nil {
		return fmt.Errorf("assume role failed: %w", err)
	}

	env := []string{
		"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
		"AWS_SESSION_TOKEN=" + creds.SessionToken,
		"AWS_REGION=" + creds.Region,
	}

	log.Printf("run %d: running terraform plan in %s as assumed role in region %s", job.RunID, modulePath, creds.Region)

	// Context with timeout for terraform commands
	tctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// 1) terraform init
	if err := runTerraformCmd(tctx, modulePath, env, "init", "-input=false", "-no-color"); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Build -var arguments from job.Inputs
	varArgs := []string{}
	for k, v := range job.Inputs {
		varArgs = append(varArgs, "-var", fmt.Sprintf("%s=%v", k, v))
	}

	// 2) terraform plan
	args := append([]string{"plan", "-input=false", "-no-color"}, varArgs...)
	if err := runTerraformCmd(tctx, modulePath, env, args...); err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}

	return nil
}

func runTerraformApply(ctx context.Context, job *Job) error {
	modulePath, err := modulePathFor(job.BlueprintKey)
	if err != nil {
		return err
	}

	// Assume role for this job (using values from job.AWS)
	creds, err := assumeRoleForJob(ctx, job)
	if err != nil {
		return fmt.Errorf("assume role failed: %w", err)
	}

	env := []string{
		"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
		"AWS_SESSION_TOKEN=" + creds.SessionToken,
		"AWS_REGION=" + creds.Region,
	}

	log.Printf("run %d: running terraform apply in %s as assumed role in region %s", job.RunID, modulePath, creds.Region)

	// Context with timeout for terraform commands
	tctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// 1) terraform init
	if err := runTerraformCmd(tctx, modulePath, env, "init", "-input=false", "-no-color"); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Build -var arguments from job.Inputs
	varArgs := []string{}
	for k, v := range job.Inputs {
		varArgs = append(varArgs, "-var", fmt.Sprintf("%s=%v", k, v))
	}

	// 2) terraform apply -auto-approve
	args := append([]string{"apply", "-input=false", "-auto-approve", "-no-color"}, varArgs...)
	if err := runTerraformCmd(tctx, modulePath, env, args...); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	return nil
}

// captureTerraformOutputs runs `terraform output -json` in the module
// directory using the same assumed role and returns the raw JSON bytes.
func captureTerraformOutputs(ctx context.Context, job *Job) ([]byte, error) {
	modulePath, err := modulePathFor(job.BlueprintKey)
	if err != nil {
		return nil, err
	}

	creds, err := assumeRoleForJob(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("assume role for outputs failed: %w", err)
	}

	env := []string{
		"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
		"AWS_SESSION_TOKEN=" + creds.SessionToken,
		"AWS_REGION=" + creds.Region,
	}

	tctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	args := []string{"-chdir=" + modulePath, "output", "-json"}
	cmd := exec.CommandContext(tctx, "terraform", args...)
	cmd.Env = append(os.Environ(), env...)

	log.Printf("exec: terraform %v", args)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform output -json failed: %w", err)
	}

	return out, nil
}

// persistOutputsForRun finds the deployment for the given run and updates
// deployments.outputs_json with the given JSON blob.
func persistOutputsForRun(ctx context.Context, db *sql.DB, runID int64, outputsJSON []byte) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var deploymentID int64
	if err := db.QueryRowContext(ctx2,
		"SELECT deployment_id FROM runs WHERE id = ?",
		runID,
	).Scan(&deploymentID); err != nil {
		return fmt.Errorf("lookup deployment_id for run %d: %w", runID, err)
	}

	_, err := db.ExecContext(ctx2,
		"UPDATE deployments SET outputs_json = ? WHERE id = ?",
		string(outputsJSON),
		deploymentID,
	)
	if err != nil {
		return fmt.Errorf("update deployments.outputs_json for deployment %d: %w", deploymentID, err)
	}

	return nil
}

func runTerraformCmd(ctx context.Context, modulePath string, extraEnv []string, args ...string) error {
	// We use -chdir so we don't have to change the worker's working directory
	allArgs := append([]string{"-chdir=" + modulePath}, args...)
	cmd := exec.CommandContext(ctx, "terraform", allArgs...)

	// Process environment: inherit + override AWS credentials
	cmd.Env = append(os.Environ(), extraEnv...)

	// Stream terraform stdout/stderr into worker logs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("exec: terraform %v", allArgs)

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func modulePathFor(blueprintKey string) (string, error) {
	// Worker is run from apps/worker, so modules are at ../../infra/modules/<name>
	const modulesRoot = "../../infra/modules"

	switch blueprintKey {
	case "ecs-service":
		return filepath.Join(modulesRoot, "ecs-service"), nil
	default:
		return "", fmt.Errorf("unsupported blueprint %q", blueprintKey)
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---- AssumeRole helpers ---------------------------------------------------

func assumeRoleForJob(ctx context.Context, job *Job) (*AWSCredentials, error) {
	roleArn := job.AWS["roleArn"]
	externalID := job.AWS["externalId"]
	region := job.AWS["region"]
	if region == "" {
		region = "ap-northeast-1"
	}

	if roleArn == "" || externalID == "" {
		return nil, fmt.Errorf("missing roleArn or externalId in job.AWS")
	}

	// Load platform credentials (from AWS_PROFILE / env)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load default AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)

	dur := int32(3600)
	out, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         &roleArn,
		RoleSessionName: awsString(fmt.Sprintf("aip-run-%d", job.RunID)),
		ExternalId:      &externalID,
		DurationSeconds: &dur,
	})
	if err != nil {
		return nil, fmt.Errorf("STS AssumeRole error: %w", err)
	}

	if out.Credentials == nil {
		return nil, fmt.Errorf("STS AssumeRole returned nil credentials")
	}

	return &AWSCredentials{
		AccessKeyID:     *out.Credentials.AccessKeyId,
		SecretAccessKey: *out.Credentials.SecretAccessKey,
		SessionToken:    *out.Credentials.SessionToken,
		Region:          region,
	}, nil
}

func awsString(s string) *string { return &s }

// ---- run status helpers ---------------------------------------------------

func markRunRunning(ctx context.Context, db *sql.DB, runID int64) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx2, `
        UPDATE runs
        SET status = 'running',
            started_at = IFNULL(started_at, NOW())
        WHERE id = ?
    `, runID)
	return err
}

func markRunSucceeded(ctx context.Context, db *sql.DB, runID int64) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx2, `
        UPDATE runs
        SET status = 'succeeded',
            finished_at = IFNULL(finished_at, NOW())
        WHERE id = ?
    `, runID)
	return err
}

func markRunFailed(ctx context.Context, db *sql.DB, runID int64, summary string) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx2, `
        UPDATE runs
        SET status = 'failed',
            summary = ?,
            finished_at = NOW()
        WHERE id = ?
    `, summary, runID)
	return err
}
