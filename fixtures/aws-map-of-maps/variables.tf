variable "region" {}

variable "env_name" {}

variable "bucket" {}
variable "object" {
  type = map
}
variable "map_of_maps" {
  type = map(map(string))
}
