module "test_module_1" {
  source = "../aws"

  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
  env_name = "${var.env_name}"

  bucket = "${var.bucket}"
  object_key = "${var.object_key}"
  object_content = "${var.object_content}"
}

module "test_module_2" {
  source = "../aws"

  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
  env_name = "${var.env_name}"

  bucket = "${var.bucket}"
  object_key = "${var.object_key}"
  object_content = "${var.object_content}"
}
