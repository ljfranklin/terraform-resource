provider "aws" {
    access_key = "${var.access_key}"
    secret_key = "${var.secret_key}"
    region = "${var.region}"
}

resource "aws_subnet" "test_subnet" {
    vpc_id = "${var.vpc_id}"
    cidr_block = "${var.subnet_cidr}"

    tags {
        Name = "${var.tag_name}"
    }
}
