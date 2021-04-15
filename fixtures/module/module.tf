module "test_module" {
  source = "../aws"

  access_key = var.access_key
  secret_key = var.secret_key
  region = var.region
  env_name = var.env_name

  build_id = var.build_id
  build_name = var.build_name
  build_job_name = var.build_job_name
  build_pipeline_name = var.build_pipeline_name
  build_team_name = var.build_team_name
  atc_external_url = var.atc_external_url

  bucket = var.bucket
  object_key = var.object_key
  object_content = var.object_content
}
