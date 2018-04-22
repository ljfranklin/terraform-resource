output "env_name" {
    value = "${module.test_module.env_name}"
}
output "bucket" {
    value = "${module.test_module.bucket}"
}
output "object_key" {
    value = "${module.test_module.object_key}"
}
output "object_content" {
    value = "${module.test_module.object_content}"
}
output "content_md5" {
    value = "${module.test_module.content_md5}"
}