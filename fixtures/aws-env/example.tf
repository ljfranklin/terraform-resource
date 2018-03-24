# expect creds to be passed via ENV variables
provider "aws" {
  region     = "${var.region}"
}

resource "aws_s3_bucket_object" "s3_object" {
  key        = "${var.object_key}"
  bucket     = "${var.bucket}"
  content    = "${var.object_content}"
}