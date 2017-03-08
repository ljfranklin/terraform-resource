output "env_name" {
    value = "${var.env_name}"
}
output "bucket" {
    value = "${var.bucket}"
}
output "object_key" {
    value = "${aws_s3_bucket_object.s3_object.id}"
}
output "object_content" {
    value = "${var.object_content}"
}
output "content_md5" {
    value = "${aws_s3_bucket_object.s3_object.etag}"
}
output "map" {
    value = "${map(
      "key-1", "value-1",
      "key-2", "value-2"
    )}"
}
output "list" {
    value = ["item-1", "item-2"]
}
output "secret" {
    sensitive = true
    value     = "super-secret"
}
