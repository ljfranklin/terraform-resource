terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
  required_version = ">= 0.13"
}

provider "aws" {
  access_key = var.access_key
  secret_key = var.secret_key
  region     = var.region
}

resource "aws_s3_bucket" "bucket" {
  bucket = var.bucket
}

resource "aws_s3_bucket_object" "s3_object" {
  bucket     = aws_s3_bucket.bucket.bucket
  key        = var.object_key
  content    = var.object_content
  # TODO: Terraform 0.14.0 returns stale etag value
  etag       = md5(var.object_content)
}
