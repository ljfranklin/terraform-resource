provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region     = "${var.region}"
}

resource "aws_s3_bucket" "bucket" {
  bucket = "${var.bucket}"
}

resource "aws_s3_bucket_object" "s3_object" {
  bucket     = "${aws_s3_bucket.bucket.bucket}"
  key        = "${var.object_key}"
  content    = "${var.object_content}"
}
