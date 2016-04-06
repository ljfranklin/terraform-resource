provider "aws" {
    access_key = "${var.access_key}"
    secret_key = "${var.secret_key}"
    region = "${var.region}"
}

resource "aws_vpc" "test_vpc" {
    cidr_block = "10.0.99.0/24"

    tags {
        Name = "${var.tag_name}"
    }
}
