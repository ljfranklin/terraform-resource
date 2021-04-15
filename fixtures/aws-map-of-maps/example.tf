terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
  required_version = ">= 0.13"
}

# expect creds to be passed via ENV variables
provider "aws" {
  region     = var.region
}

resource "aws_s3_bucket_object" "s3_object" {
  key        = var.object["key"]
  bucket     = var.bucket
  content    = var.object["content"]
  # TODO: Terraform 0.14.0 returns stale etag value
  etag       = md5(var.object["content"])
}
