module "test_module" {
  source = "github.com/ljfranklin/terraform-resource//fixtures/aws"

  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
  vpc_id = "${var.vpc_id}"
  subnet_cidr = "${var.subnet_cidr}"
  tag_name = "${var.tag_name}"
  env_name = "${var.env_name}"
}
