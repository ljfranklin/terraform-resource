provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region     = "${var.region}"
}

resource "aws_s3_bucket_object" "s3_object" {
  key        = "${var.object_key}"
  bucket     = "${var.bucket}"
  content    = "${var.object_content}"
  acl        = "${var.object_acl}"
}
