package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Job struct {
	RunID        int64             `json:"run_id"`
	Action       string            `json:"action"`
	BlueprintKey string            `json:"blueprint_key"`
	Version      string            `json:"version"`
	Inputs       map[string]any    `json:"inputs"`
	AWS          map[string]string `json:"aws"`
}

func main() {
	addr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()

	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	log.Printf("worker listening on queue aip:jobs (redis=%s)", addr)

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

		if err := handleJob(ctx, &job); err != nil {
			log.Printf("job %d failed: %v", job.RunID, err)
		} else {
			log.Printf("job %d completed successfully", job.RunID)
		}
	}
}

func handleJob(ctx context.Context, job *Job) error {
	switch job.Action {
	case "plan":
		return runTerraformPlan(ctx, job)
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

	log.Printf("run %d: running terraform plan in %s", job.RunID, modulePath)

	// Create a context with timeout for terraform commands
	tctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// 1) terraform init
	if err := runTerraformCmd(tctx, modulePath, "init", "-input=false", "-no-color"); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Build -var arguments from job.Inputs
	varArgs := []string{}
	for k, v := range job.Inputs {
		// we rely on the module variable names matching the JSON keys
		varArgs = append(varArgs, "-var", fmt.Sprintf("%s=%v", k, v))
	}

	// 2) terraform plan
	args := append([]string{"plan", "-input=false", "-no-color"}, varArgs...)
	if err := runTerraformCmd(tctx, modulePath, args...); err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}

	return nil
}

func runTerraformCmd(ctx context.Context, modulePath string, args ...string) error {
	// We use -chdir so we don't have to change the worker's working directory
	allArgs := append([]string{"-chdir=" + modulePath}, args...)
	cmd := exec.CommandContext(ctx, "terraform", allArgs...)

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
