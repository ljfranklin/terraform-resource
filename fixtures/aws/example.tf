provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region     = "${var.region}"
}

resource "aws_s3_bucket_object" "s3_object" {
  key        = "${var.object_key}"
  bucket     = "${var.bucket}"
  content    = "${var.object_content}"
}

# used to verify error handling
resource "aws_s3_bucket_object" "invalid_object" {
  count      = "${var.invalid_object_count}"
  # ensure partially created resources
  depends_on = ["aws_s3_bucket_object.s3_object"]

  key        = "${var.object_key}-acl"
  bucket     = "${var.bucket}"
  content    = "${var.object_content}"
  kms_key_id = "arn:aws:kms:us-east-1:111111111111:key/INVALID_KEY"
}
