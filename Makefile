SHELL := /usr/bin/env bash

PROJECT_ROOT := $(CURDIR)
API_DIR := apps/api
WORKER_DIR := apps/worker
WEB_DIR := apps/web
PLATFORM_INFRA_DIR := infra/platform
TARGET_INFRA_DIR := infra/target-dev

API_PORT ?= 8080
MYSQL_DSN ?= aip:aip@tcp(127.0.0.1:3306)/aws_infra_platform?parseTime=true&multiStatements=true
REDIS_ADDR ?= 127.0.0.1:6379
COGNITO_JWKS_URL ?= https://cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_rpMCl430S/.well-known/jwks.json

AWS_PROFILE ?= aip-platform
DB_URL := mysql://aip:aip@tcp(127.0.0.1:3306)/aws_infra_platform

.PHONY: help docker-up docker-down logs \
        api-dev worker-dev web-dev \
        migrate-up migrate-down \
        tf-platform-plan tf-platform-apply \
        tf-target-plan tf-target-apply

help: ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sed -E 's/:.*##/ -/' | sort

# ----- Docker & infra ------------------------------------------------------

docker-up: ## Start local MySQL + Redis via docker compose
	docker compose up -d

docker-down: ## Stop local MySQL + Redis
	docker compose down

logs: ## Tail docker compose logs
	docker compose logs -f

# ----- API / worker / web --------------------------------------------------

api-dev: ## Run API in dev mode
	cd $(API_DIR) && \
	API_PORT=$(API_PORT) \
	MYSQL_DSN='$(MYSQL_DSN)' \
	REDIS_ADDR=$(REDIS_ADDR) \
	COGNITO_JWKS_URL=$(COGNITO_JWKS_URL) \
	go run cmd/api/main.go

worker-dev: ## Run worker in dev mode
	cd $(WORKER_DIR) && \
	REDIS_ADDR=$(REDIS_ADDR) \
	go run cmd/worker/main.go

web-dev: ## Run Next.js dev server
	cd $(WEB_DIR) && \
	npm run dev

# ----- DB migrations -------------------------------------------------------

migrate-up: ## Apply DB migrations (up)
	migrate -path $(API_DIR)/db/migrations -database "$(DB_URL)" up

migrate-down: ## Roll back last DB migration (down)
	migrate -path $(API_DIR)/db/migrations -database "$(DB_URL)" down 1

# ----- Terraform: platform (Cognito, etc.) ---------------------------------

tf-platform-plan: ## terraform plan for platform infra
	cd $(PLATFORM_INFRA_DIR) && \
	AWS_PROFILE=$(AWS_PROFILE) terraform plan

tf-platform-apply: ## terraform apply for platform infra
	cd $(PLATFORM_INFRA_DIR) && \
	AWS_PROFILE=$(AWS_PROFILE) terraform apply

# ----- Terraform: target-dev (deploy role) ---------------------------------

tf-target-plan: ## terraform plan for target-dev infra
	cd $(TARGET_INFRA_DIR) && \
	AWS_PROFILE=$(AWS_PROFILE) terraform plan

tf-target-apply: ## terraform apply for target-dev infra
	cd $(TARGET_INFRA_DIR) && \
	AWS_PROFILE=$(AWS_PROFILE) terraform apply
