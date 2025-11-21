variable "serviceName" {
  type        = string
  description = "Name of the ECS service and related resources"
}

variable "image" {
  type        = string
  description = "Container image"
}

variable "cpu" {
  type        = number
  description = "CPU units for the task definition"
}

variable "memory" {
  type        = number
  description = "Memory (MiB) for the task definition"
}
