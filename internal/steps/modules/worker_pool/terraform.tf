terraform {
  required_providers {
    octopusdeploy = { source = "OctopusDeploy/octopusdeploy", version = "1.3.10" }
    // Use the option below when debugging
    // octopusdeploy = { source = "octopus.com/com/octopusdeploy" }
  }
}

provider "octopusdeploy" {
  address  = var.octopus_server
  api_key  = var.octopus_apikey
  space_id = var.octopus_space_id
}

variable "octopus_server" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The URL of the Octopus server e.g. https://myinstance.octopus.app."
}

variable "octopus_apikey" {
  type        = string
  nullable    = false
  sensitive   = true
  description = "The API key used to access the Octopus server. See https://octopus.com/docs/octopus-rest-api/how-to-create-an-api-key for details on creating an API key."
}

variable "octopus_space_id" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The space ID to populate"
}

variable "worker_pool_name" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The space ID to populate"
}

variable "worker_pool_type" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The space ID to populate"
    default     = "WindowsDefault"
}

resource "octopusdeploy_dynamic_worker_pool" "example" {
  description = "Migrated default worker pool"
  is_default  = false
  name        = var.worker_pool_name
  sort_order  = 0
  worker_type = var.worker_pool_type
}