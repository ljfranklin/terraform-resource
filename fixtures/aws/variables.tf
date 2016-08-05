variable "access_key" {}
variable "secret_key" {}
variable "region" {
    default = "us-east-1"
}
variable "vpc_id" {}
variable "subnet_cidr" {}
variable "tag_name" {
    default = "terraform-resource-test"
}
variable "env_name" {}

# used to verify `destroy_on_failure`
variable "acl_count" {
    default = "0"
}
variable "acl_action" {
    default = "allow"
}
