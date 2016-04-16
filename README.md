## Terraform Concourse Resource

A work in-progress Concourse resource that allows jobs to modify IaaS resources via Terraform.
Examples and setup instructions coming soon.

See what's in progress on the [Trello board](https://trello.com/b/s06sLNwc/terraform-resource).

## Source Configuration

### `storage`

* `storage.driver`: *Optional. Default `s3`.* The driver used to store the Terraform state file. Currently `s3` is the only supported driver.

#### `s3` Driver

* `storage.bucket`: *Required.* The S3 bucket used to store the state file.

* `storage.key`: *Required.* The S3 object key used to store the state file.

* `storage.access_key_id`: *Required.* The AWS access key used to access the bucket.

* `storage.secret_access_key`: *Required.* The AWS secret key used to access the bucket.

### `terraform`

Terraform configuration options can be specified under `source.terraform` and/or `put.params.terraform`.
Options from these two locations will be merged, with fields under `put.params.terraform` taking precedence.

* `terraform.source`: *Required.* The location of the Terraform module to apply.
These can be local paths, URLs, GitHub repos and more.
See [Terraform Sources](https://www.terraform.io/docs/modules/sources.html) for more examples.

* `terraform.vars`: *Optional.* A collection of Terraform input variables.
These are typically used to specify credentials or override default module values.
See [Terraform Input Variables](https://www.terraform.io/intro/getting-started/variables.html) for more details.

### Example

**Note:** Declaring custom resources under `resource_types` requires Concourse 1.0 or newer.

```yaml
resource_types:
- name: terraform
  type: docker-image
  source:
    repository: ljfranklin/terraform-resource

resources:
  - name: terraform
    type: terraform
    source:
      storage:
        bucket: mybucket
        key: terraform/concourse.tfstate
        access_key_id: {{storage_access_key}}
        secret_access_key: {{storage_secret_key}}
      terraform:
        # the '//' indicates a sub-directory in a git repo
        source: github.com/ljfranklin/terraform-resource//fixtures/aws
        vars:
          access_key: {{environment_access_key}}
          secret_key: {{environment_secret_key}}
          tag_name: concourse
```

## Behavior

### `check`: Check for modifications to the state file.

Detects modifications to the state file specified under `source.storage.key`.
The last modified time of the state file is used for resource versioning.

### `in`: Provide a file containing the Terraform Outputs

Creates a `metadata` file containing the [Terraform Outputs](https://www.terraform.io/intro/getting-started/outputs.html) in JSON format.
This step expects that a previous `put` step has already created the state file.

#### Example:

```yaml
jobs:
  - name: show-outputs
    plan:
      - get: terraform
        passed: [create-infrastructure]
      - task: show-outputs
        config:
          platform: linux
          inputs:
            - name: terraform
          run:
            path: /bin/sh
            args:
              - -c
              - "cat terraform/metadata"
```

The preceding job would show a file similar to the following:

```json
{
  "vpc_id": "vpc-123456",
  "vpc_tag_name": "concourse"
}
```

### `out`: Create, Update, and Destroy IaaS resources

Depending on the context, the `out` step will perform one of three actions:

**Create:**
If the state file specified by `source.storage.key` does not exist, `out` will create all the IaaS resources specified by `terraform.source`.
It then uploads the resulting state file using the configured `storage` driver.

**Update:**
If the state file already exists, `out` will update the IaaS resources to match the desired state specified in `terraform.source`.
It then uploads the updated state file.

**Destroy:**
If the `destroy` action is specified, `out` will destroy all IaaS resources specified in the state file.
It then deletes the state file using the configured `storage` driver.

#### Parameters

The same Terraform configuration options described under `source.terraform` can also be specified under `put.params.terraform` with the following addition:

* `terraform.var_file`: *Optional.* A file containing Terraform input variables.
This file can be in YAML or JSON format.

Terraform variables will be merged from the following locations in increasing order of precedence: `source.terraform.vars`, `put.params.terraform.vars`, and `put.params.terraform.var_file`.

* `action`: *Optional.* Used to indicate a destructive `put`. The only recognized value is `destroy`, create / update are the implicit defaults.
