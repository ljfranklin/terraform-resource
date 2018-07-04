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
    value = "${var.object["content"]}"
}
output "content_md5" {
    value = "${aws_s3_bucket_object.s3_object.etag}"
}
