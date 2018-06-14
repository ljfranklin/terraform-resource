
// overrides s3_object defined in aws/example.tf
resource "aws_s3_bucket_object" "s3_object" {
  key        = "${var.object_key}"
  bucket     = "${var.bucket}"
  content    = "OVERRIDE"
}

output "object_content" {
    value = "OVERRIDE"
}
