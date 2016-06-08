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
        EnvName = "${var.env_name}"
    }
}

resource "aws_network_acl" "test_acl" {
    vpc_id = "${var.vpc_id}"
    count = "${var.acl_count}"

    egress {
        action = "${var.acl_action}"
        from_port = 0
        to_port = 65535
        protocol = "tcp"
        cidr_block = "${aws_subnet.test_subnet.cidr_block}"
        rule_no = 1
    }

    ingress {
        action = "${var.acl_action}"
        from_port = 0
        to_port = 65535
        protocol = "tcp"
        cidr_block = "${aws_subnet.test_subnet.cidr_block}"
        rule_no = 1
    }

    tags {
        Name = "${var.tag_name}"
        EnvName = "${var.env_name}"
    }
}
