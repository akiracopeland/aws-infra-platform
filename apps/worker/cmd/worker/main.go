package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type Job struct {
	RunID        int64             `json:"run_id"`
	Action       string            `json:"action"` // plan/apply/destroy
	BlueprintKey string            `json:"blueprint_key"`
	Version      string            `json:"version"`
	Inputs       map[string]any    `json:"inputs"`
	AWS          map[string]string `json:"aws"` // roleArn, externalId, region
}

func main() {
	rdb := redis.NewClient(&redis.Options{Addr: getEnv("REDIS_ADDR", "127.0.0.1:6379")})
	ctx := context.Background()
	for {
		x, err := rdb.BLPop(ctx, 5*time.Second, "aip:jobs").Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			log.Printf("redis: %v", err)
			continue
		}
		if len(x) != 2 {
			continue
		}
		var job Job
		if err := json.Unmarshal([]byte(x[1]), &job); err != nil {
			log.Printf("decode: %v", err)
			continue
		}

		log.Printf("job: run=%d action=%s blueprint=%s@%s", job.RunID, job.Action, job.BlueprintKey, job.Version)
		// TODO: dispatch: terraformPlanApply(job) or awsSDKOp(job)
	}
}
func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
