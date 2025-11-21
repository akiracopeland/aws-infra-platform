terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

provider "aws" {
  region = "ap-northeast-1"
}

data "aws_caller_identity" "current" {}

# A role the platform will assume to deploy ECS/etc into this account
resource "aws_iam_role" "aip_target_deploy" {
  name = "aip-target-deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Principal = {
          # For now: allow this same account (platform == target).
          # Later, change to the platform account ID if they differ.
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        },
        Action = "sts:AssumeRole",
        Condition = {
          # Optional defense-in-depth for real cross-account later
          StringEquals = {
            "sts:ExternalId" = "aip-dev-external-id"
          }
        }
      }
    ]
  })
}

# For MVP: broad-ish permissions so deployments don't get blocked.
# Later we can tighten this down around ECS/VPC/ALB/IAM/etc.
resource "aws_iam_role_policy_attachment" "ecs_full" {
  role       = aws_iam_role.aip_target_deploy.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonECS_FullAccess"
}

resource "aws_iam_role_policy_attachment" "ec2_full" {
  role       = aws_iam_role.aip_target_deploy.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2FullAccess"
}

resource "aws_iam_role_policy_attachment" "cw_logs_full" {
  role       = aws_iam_role.aip_target_deploy.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchLogsFullAccess"
}

# You may also need IAM pass-role and load balancer permissions later;
# we can add those as we flesh out the ECS module.

output "deploy_role_arn" {
  value = aws_iam_role.aip_target_deploy.arn
}

output "external_id" {
  value = "aip-dev-external-id"
}

resource "aws_iam_role_policy" "aip_allow_task_execution_role" {
  name = "aip-allow-task-execution-role"
  role = aws_iam_role.aip_target_deploy.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "iam:CreateRole",
          "iam:DeleteRole",
          "iam:AttachRolePolicy",
          "iam:DetachRolePolicy",
          "iam:PassRole",
          "iam:GetRole",
          "iam:ListRolePolicies",
          "iam:GetRolePolicy",
          "iam:ListInstanceProfilesForRole"
        ],
        Resource = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/demo-service-execution"
      }
    ]
  })
}
