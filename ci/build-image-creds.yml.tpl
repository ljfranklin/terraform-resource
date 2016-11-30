docker_email:             ""
docker_username:          ""
docker_password:          ""

resource_git_uri:         https://github.com/ljfranklin/terraform-resource.git
resource_git_branch:      master

terraform_git_uri:        https://github.com/hashicorp/terraform.git
terraform_git_branch:     master
# can be used to fetch RC builds - "v*-*"
terraform_git_tag_filter: *

docker_repository:        ljfranklin/terraform-resource
docker_tag:               nightly
