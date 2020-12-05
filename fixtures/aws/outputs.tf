output "env_name" {
    value = var.env_name
}
output "build_id" {
    value = var.build_id
}
output "build_name" {
    value = var.build_name
}
output "build_job_name" {
    value = var.build_job_name
}
output "build_pipeline_name" {
    value = var.build_pipeline_name
}
output "build_team_name" {
    value = var.build_team_name
}
output "atc_external_url" {
    value = var.atc_external_url
}
output "bucket" {
    value = var.bucket
}
output "object_key" {
    value = aws_s3_bucket_object.s3_object.id
}
output "object_content" {
    value = var.object_content
}
output "content_md5" {
    value = aws_s3_bucket_object.s3_object.etag
}
output "map" {
    value = map(
      "key-1", "value-1",
      "key-2", "value-2"
    )
}
output "list" {
    value = ["item-1", "item-2"]
}
output "secret" {
    sensitive = true
    value     = "super-secret"
}
