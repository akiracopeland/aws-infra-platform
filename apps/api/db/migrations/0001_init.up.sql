-- Users & Orgs
CREATE TABLE users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL UNIQUE,
  name VARCHAR(255) NULL,
  cognito_sub VARCHAR(64) NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orgs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE memberships (
  user_id BIGINT NOT NULL,
  org_id BIGINT NOT NULL,
  role ENUM('admin','maintainer','operator','viewer') NOT NULL DEFAULT 'viewer',
  PRIMARY KEY (user_id, org_id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES orgs(id)
);

-- Projects & Envs
CREATE TABLE projects (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  org_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  UNIQUE(org_id, name),
  FOREIGN KEY (org_id) REFERENCES orgs(id)
);

CREATE TABLE environments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  project_id BIGINT NOT NULL,
  name ENUM('dev','stg','prod') NOT NULL,
  UNIQUE(project_id, name),
  FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- AWS connections
CREATE TABLE aws_connections (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  org_id BIGINT NOT NULL,
  account_id VARCHAR(12) NOT NULL,
  role_arn VARCHAR(2048) NOT NULL,
  external_id VARCHAR(128) NOT NULL,
  nickname VARCHAR(255) NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (org_id) REFERENCES orgs(id),
  FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Blueprints & Deployments
CREATE TABLE blueprints (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  blueprint_key VARCHAR(128) NOT NULL,  -- e.g., 'ecs-service'
  version VARCHAR(32) NOT NULL,         -- '1.0.0'
  provider VARCHAR(64) NOT NULL,        -- 'aws'
  schema_uri VARCHAR(512) NOT NULL,
  module_ref VARCHAR(512) NOT NULL,
  deprecated_at TIMESTAMP NULL,
  UNIQUE(blueprint_key, version)
);

CREATE TABLE deployments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  blueprint_id BIGINT NOT NULL,
  environment_id BIGINT NOT NULL,
  status ENUM('pending','planning','planned','applying','applied','failed','destroying','destroyed') NOT NULL DEFAULT 'pending',
  inputs_json JSON NOT NULL,
  outputs_json JSON NULL,
  cost_estimate DECIMAL(10,2) NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (blueprint_id) REFERENCES blueprints(id),
  FOREIGN KEY (environment_id) REFERENCES environments(id),
  FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE runs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  deployment_id BIGINT NOT NULL,
  action ENUM('plan','apply','destroy','drift') NOT NULL,
  status ENUM('queued','running','succeeded','failed') NOT NULL,
  artifacts_uri VARCHAR(1024) NULL, -- s3://.../plan
  summary TEXT NULL,
  started_at TIMESTAMP NULL,
  finished_at TIMESTAMP NULL,
  FOREIGN KEY (deployment_id) REFERENCES deployments(id)
);

CREATE TABLE resources (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  deployment_id BIGINT NOT NULL,
  arn VARCHAR(2048) NOT NULL,
  type VARCHAR(128) NOT NULL,
  name VARCHAR(255) NULL,
  region VARCHAR(64) NULL,
  tags_json JSON NULL,
  -- Index only the first 255 chars of arn to stay under the key length limit
  UNIQUE KEY uniq_deployment_arn (deployment_id, arn(255)),
  FOREIGN KEY (deployment_id) REFERENCES deployments(id)
);

CREATE TABLE audit_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_id BIGINT NOT NULL,
  verb VARCHAR(64) NOT NULL,  -- 'CREATE', 'APPLY', 'DESTROY', ...
  object VARCHAR(64) NOT NULL,
  object_id BIGINT NULL,
  metadata_json JSON NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (actor_id) REFERENCES users(id)
);

CREATE TABLE budgets (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  environment_id BIGINT NOT NULL,
  monthly_limit_usd DECIMAL(10,2) NOT NULL,
  alert_thresholds_json JSON NOT NULL,
  FOREIGN KEY (environment_id) REFERENCES environments(id)
);
