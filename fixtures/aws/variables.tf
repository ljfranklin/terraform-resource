variable "access_key" {}
variable "secret_key" {}
variable "region" {
    default = "us-east-1"
}
variable "env_name" {}
variable "build_id" {
    ephemeral = true
}
variable "build_name" {
    ephemeral = true
}
variable "build_job_name" {
    ephemeral = true
}
variable "build_pipeline_name" {
    ephemeral = true
}
variable "build_team_name" {
    ephemeral = true
}
variable "atc_external_url" {
    ephemeral = true
}

variable "bucket" {}
variable "object_key" {}
variable "object_content" {}

# used to verify error handling
variable "invalid_object_count" {
    default = 0
}
