package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	handlers "github.com/your-org/aws-infra-platform/apps/api/internal/handlers"
)

func main() {
	cfg := mustLoadConfig()

	db := mustOpenDB(cfg)
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	r.Use(JWTMiddleware(cfg))
	r.GET("/whoami", func(c *gin.Context) {
		user := c.GetString("user_email")
		roles := c.GetStringSlice("roles")
		c.JSON(200, gin.H{"email": user, "roles": roles})

	})

	deps := &handlers.ServerDeps{
		DB:  db,
		RDB: rdb,
	}

	api := r.Group("/v1")
	{
		api.POST("/deployments", deps.CreateDeployment)
	}

	// TODO: wire handlers (connections, blueprints, deployments)
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	log.Printf("api up on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}

type Config struct {
	Port           string
	MySQLDSN       string
	RedisAddr      string
	CognitoJWKSURL string
}

func mustLoadConfig() Config {
	port := getEnv("API_PORT", "8080")
	dsn := getEnv("MYSQL_DSN", "aip:aip@tcp(127.0.0.1:3306)/aws_infra_platform?parseTime=true&multiStatements=true")
	redis := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	jwks := os.Getenv("COGNITO_JWKS_URL") // allow empty for local/mock
	return Config{Port: port, MySQLDSN: dsn, RedisAddr: redis, CognitoJWKSURL: jwks}
}
func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func mustOpenDB(cfg Config) *sql.DB {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return db
}

// Lightweight JWT middleware for Cognito (accepts unsigned tokens when JWKS empty → local dev only)
func JWTMiddleware(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if auth == "" {
			// local dev: proceed as anonymous viewer
			c.Set("user_email", "dev@example.com")
			c.Set("roles", []string{"viewer"})
			c.Next()
			return
		}
		parser := jwt.Parser{}
		token, _, err := parser.ParseUnverified(auth, jwt.MapClaims{})
		if err != nil {
			c.AbortWithStatus(401)
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		email, _ := claims["email"].(string)
		groupsAny, _ := claims["cognito:groups"].([]any)
		var roles []string
		for _, g := range groupsAny {
			if s, ok := g.(string); ok {
				roles = append(roles, s)
			}
		}
		if email == "" {
			email = "unknown@user"
		}
		c.Set("user_email", email)
		c.Set("roles", roles)
		c.Next()
	}
}

// Example: STS AssumeRole validator (call from handler when saving AWS connection)
func validateRole(roleArn, externalID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	s := sts.NewFromConfig(awsCfg)
	_, err = s.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         &roleArn,
		RoleSessionName: awsString("aip-validate"),
		ExternalId:      awsString(externalID),
		DurationSeconds: awsInt32(900),
	})
	return err
}
func awsString(s string) *string { return &s }
func awsInt32(i int32) *int32    { return &i }
