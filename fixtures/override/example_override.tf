
// overrides s3_object defined in aws/example.tf
resource "aws_s3_bucket_object" "s3_object" {
  key        = var.object_key
  bucket     = var.bucket
  content    = "OVERRIDE"
  # TODO: Terraform 0.14.0 returns stale etag value
  etag       = md5("OVERRIDE")
}

output "object_content" {
    value = "OVERRIDE"
}
