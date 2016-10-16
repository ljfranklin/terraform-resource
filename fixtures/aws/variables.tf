variable "access_key" {}
variable "secret_key" {}
variable "region" {
    default = "us-east-1"
}
variable "env_name" {}

variable "bucket" {}
variable "object_key" {}
variable "object_content" {}

# used to verify error handling
variable "invalid_object_count" {
    default = 0
}
