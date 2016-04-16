output "vpc_id" {
    value = "${aws_vpc.test_vpc.id}"
}
output "vpc_tag_name" {
    value = "${aws_vpc.test_vpc.tags.Name}"
}
