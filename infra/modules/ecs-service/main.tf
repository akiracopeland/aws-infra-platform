terraform {
  required_version = ">= 1.6.0"
}

# Match the keys we send from the API/worker:
#   serviceName, image, cpu, memory
variable "serviceName" {
  type = string
}

variable "image" {
  type = string
}

variable "cpu" {
  type = number
}

variable "memory" {
  type = number
}

# No real AWS resources yet.
# Just an output so we see something in the plan.
output "summary" {
  value = {
    serviceName = var.serviceName
    image       = var.image
    cpu         = var.cpu
    memory      = var.memory
  }
}
