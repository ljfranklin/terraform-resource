output "env_name" {
    value = var.env_name
}
output "build_id" {
    value = var.build_id
    ephemeral = true
}
output "build_name" {
    value = var.build_name
    ephemeral = true
}
output "build_job_name" {
    value = var.build_job_name
    ephemeral = true
}
output "build_pipeline_name" {
    value = var.build_pipeline_name
    ephemeral = true
}
output "build_team_name" {
    value = var.build_team_name
    ephemeral = true
}
output "atc_external_url" {
    value = var.atc_external_url
    ephemeral = true
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
    value = tomap({
      "key-1" = "value-1",
      "key-2" = "value-2"
    })
}
output "list" {
    value = ["item-1", "item-2"]
}
output "secret" {
    sensitive = true
    value     = "super-secret"
}
