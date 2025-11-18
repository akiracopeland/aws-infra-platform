terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "aws" {
  region = "ap-northeast-1"
}

# Random suffix so the Cognito domain is globally unique
resource "random_id" "suffix" {
  byte_length = 3
}

# Cognito User Pool
resource "aws_cognito_user_pool" "aip" {
  name = "aip-user-pool"

  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = true
    mutable             = false
  }

  auto_verified_attributes = ["email"]

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }
}

# Cognito Domain (hosted by Cognito)
resource "aws_cognito_user_pool_domain" "aip" {
  domain       = "aip-${random_id.suffix.hex}"
  user_pool_id = aws_cognito_user_pool.aip.id
}

# Web app client (Next.js frontend)
resource "aws_cognito_user_pool_client" "web" {
  name         = "aip-web"
  user_pool_id = aws_cognito_user_pool.aip.id

  callback_urls = [
    "http://localhost:3000",
  ]

  logout_urls = [
    "http://localhost:3000",
  ]

  allowed_oauth_flows_user_pool_client = true

  allowed_oauth_flows = [
    "code",
  ]

  allowed_oauth_scopes = [
    "openid",
    "email",
    "profile",
  ]

  supported_identity_providers = [
    "COGNITO",
  ]

  generate_secret = false

  prevent_user_existence_errors = "ENABLED"
}

# Outputs to wire into web/API
output "user_pool_id" {
  value = aws_cognito_user_pool.aip.id
}

output "user_pool_client_id" {
  value = aws_cognito_user_pool_client.web.id
}

output "user_pool_domain" {
  value = aws_cognito_user_pool_domain.aip.domain
}
