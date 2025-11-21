output "service_id" {
  value       = aws_ecs_service.this.id
  description = "ID of the ECS service (ARN-like identifier)"
}

output "cluster_id" {
  value       = aws_ecs_cluster.this.id
  description = "ID of the ECS cluster"
}

output "log_group" {
  value       = aws_cloudwatch_log_group.this.name
  description = "CloudWatch Logs group name"
}

# For now we don't have a load balancer; keep lb_url null.
output "lb_url" {
  value       = null
  description = "Load balancer URL (not used yet)"
}

output "summary" {
  value = {
    serviceName = var.serviceName
    image       = var.image
    cpu         = var.cpu
    memory      = var.memory
    clusterId   = aws_ecs_cluster.this.id
    serviceId   = aws_ecs_service.this.id
    logGroup    = aws_cloudwatch_log_group.this.name
  }
}
