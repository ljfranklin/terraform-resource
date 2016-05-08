output "vpc_id" {
    value = "${module.test_module.vpc_id}"
}
output "subnet_id" {
    value = "${module.test_module.subnet_id}"
}
output "subnet_cidr" {
    value = "${module.test_module.subnet_cidr}"
}
output "tag_name" {
    value = "${module.test_module.tag_name}"
}
output "env_name" {
    value = "${module.test_module.env_name}"
}
