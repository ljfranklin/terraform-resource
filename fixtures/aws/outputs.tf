output "vpc_id" {
    value = "${aws_vpc.test_vpc.id}"
}
output "vpc_tags" {
    value = "${aws_vpc.test_vpc.tags}"
}
