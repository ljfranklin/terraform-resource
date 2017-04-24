output "env_name_1" {
    value = "${module.test_module_1.env_name}"
}
output "bucket_1" {
    value = "${module.test_module_1.bucket}"
}
output "object_key_1" {
    value = "${module.test_module_1.object_key}"
}
output "object_content_1" {
    value = "${module.test_module_1.object_content}"
}
output "content_md5_1" {
    value = "${module.test_module_1.content_md5}"
}
output "env_name_2" {
    value = "${module.test_module_2.env_name}"
}
output "bucket_2" {
    value = "${module.test_module_2.bucket}"
}
output "object_key_2" {
    value = "${module.test_module_2.object_key}"
}
output "object_content_2" {
    value = "${module.test_module_2.object_content}"
}
output "content_md5_2" {
    value = "${module.test_module_2.content_md5}"
}