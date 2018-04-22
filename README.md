## Terraform Concourse Resource

A [Concourse](http://concourse.ci/) resource that allows jobs to modify IaaS resources via [Terraform](https://www.terraform.io/).
Useful for creating a pool of reproducible environments. No more snowflakes!

See [DEVELOPMENT](DEVELOPMENT.md) if you're interested in submitting a PR :+1:

![Docker Pulls](https://img.shields.io/docker/pulls/ljfranklin/terraform-resource.svg)

## Source Configuration

> **Note:** If you need to store Terraform state file in a service other than S3, try the [backend-beta image](https://github.com/ljfranklin/terraform-resource/tree/WIP-tf-backends#backend-beta).

* `storage.driver`: *Optional. Default `s3`.* The driver used to store the Terraform state file. Currently `s3` is the only supported driver.

* `storage.bucket`: *Required.* The S3 bucket used to store the state files.

* `storage.bucket_path`: *Required.* The S3 path used to store state files, e.g. `terraform-ci/`.

* `storage.access_key_id`: *Required.* The AWS access key used to access the bucket.

* `storage.secret_access_key`: *Required.* The AWS secret key used to access the bucket.

* `storage.region_name`: *Optional.* The AWS region where the bucket is located.

* `storage.server_side_encryption`: *Optional.* An encryption algorithm to use when storing objects in S3, e.g. "AES256".

* `storage.sse_kms_key_id` *Optional.* The ID of the AWS KMS master encryption key used for the object.

* `storage.endpoint`: *Optional.* The endpoint for an s3-compatible blobstore (e.g. Ceph).

  > **Note:** By default, the resource will use S3 signing version v2 if an endpoint is specified as many non-S3 blobstores do not support v4.
Opt into v4 signing by setting `storage.use_signing_v4: true`.

* `delete_on_failure`: *Optional. Default `false`.* If true, the resource will run `terraform destroy` if `terraform apply` returns an error.

* `vars`: *Optional.* A collection of Terraform input variables.
These are typically used to specify credentials or override default module values.
See [Terraform Input Variables](https://www.terraform.io/intro/getting-started/variables.html) for more details.
Since Concourse currently only supports [interpolating strings](https://github.com/concourse/concourse/issues/545) into the pipeline config, you may need to use Terraform helpers like [split](https://www.terraform.io/docs/configuration/interpolation.html#split_delim_string_) to handle lists and maps as inputs.

* `env`: *Optional.* Similar to `vars`, this collection of key-value pairs can be used to pass environment variables to Terraform, e.g. "AWS_ACCESS_KEY_ID".

* `ssh_private_key`: *Optional.* An SSH key used to fetch modules, e.g. [private GitHub repos](https://www.terraform.io/docs/modules/sources.html#private-github-repos).

#### Source Example

> **Note:** Declaring custom resources under `resource_types` requires Concourse 1.0 or newer.

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
        bucket_path: terraform-ci/
        access_key_id: {{storage_access_key}}
        secret_access_key: {{storage_secret_key}}
      vars:
        tag_name: concourse
      env:
        AWS_ACCESS_KEY_ID: {{environment_access_key}}
        AWS_SECRET_ACCESS_KEY: {{environment_secret_key}}
```

#### Image Variants

- Latest tagged release of Terraform: `ljfranklin/terraform-resource:latest`.
- Specific versions of Terraform, e.g. `ljfranklin/terraform-resource:0.7.7`.
- [RC builds](https://concourse.lylefranklin.com/teams/main/pipelines/terraform-resource-rc) from Terraform pre-releases: `ljfranklin/terraform-resource:rc`.
- [Nightly builds](https://concourse.lylefranklin.com/teams/main/pipelines/terraform-resource-nightly) from Terraform `master` branch: `ljfranklin/terraform-resource:nightly`.

See [Dockerhub](https://hub.docker.com/r/ljfranklin/terraform-resource/tags/) for a list of all available tags.
If you'd like to build your own image from a specific Terraform branch, configure a pipeline with [build-image-pipeline.yml](ci/build-image-pipeline.yml).

## Behavior

This resource should usually be used with the `put` action rather than a `get`.
This ensures the output always reflects the current state of the IaaS and allows management of multiple environments as shown below. A `get` step takes no parameters and outputs the same `metadata` file format shown below for `put`.

Depending on the context, the `put` step will perform one of three actions:

**Create:**
If the state file does not exist, `put` will create all the IaaS resources specified by `terraform_source`.
It then uploads the resulting state file using the configured `storage` driver.

**Update:**
If the state file already exists, `put` will update the IaaS resources to match the desired state specified in `terraform_source`.
It then uploads the updated state file.

**Destroy:**
If the `destroy` action is specified, `put` will destroy all IaaS resources specified in the state file.
It then deletes the state file using the configured `storage` driver.

#### Get Parameters

> **Note:** In Concourse, a `put` is always followed by an implicit `get`. To pass `get` params via `put`, use `put.get_params`.

* `output_statefile`: *Optional. Default `false`* If true, the resource writes the Terraform statefile to a file named `terraform.tfstate`. **Warning:** Ensure any changes to this statefile are persisted back to the resource's storage bucket. **Another warning:** Some statefiles contain unencrypted secrets, be careful not to expose these in your build logs.

* `output_module` *Optional.* Write only the outputs from the given module name to the `metadata` file.

#### Put Parameters

* `terraform_source`: *Required.* The relative path of the directory containing your Terraform configuration files.
For example: if your `.tf` files are stored in a git repo called `prod-config` under a directory `terraform-configs`, you could do a `get: prod-config` in your pipeline with `terraform_source: prod-config/terraform-configs/` as the source.

* `env_name`: *Optional.* The name of the environment to create or modify. Multiple environments can be managed with a single resource.

* `generate_random_name`: *Optional. Default `false`* Generates a random `env_name` (e.g. "coffee-bee"). Useful for creating lock files.

* `env_name_file`: *Optional.* Reads the `env_name` from a specified file path. Useful for destroying environments from a lock file.

* `delete_on_failure`: *Optional. Default `false`.* See description under `source.delete_on_failure`.

* `vars`: *Optional.* A collection of Terraform input variables. See description under `source.vars`.

* `var_files`: *Optional.* A list of files containing Terraform input variables. These files can be in YAML or JSON format.

  > Terraform variables will be merged from the following locations in increasing order of precedence: `source.vars`, `put.params.vars`, and `put.params.var_files`. If a state file already exists, the outputs will be fed back in as input `vars` to subsequent `puts` with the lowest precedence.
Finally, `env_name` is automatically passed as an input `var`.

* `env`: *Optional.* A key-value collection of environment variables to pass to Terraform. See description under `source.env`.

* `ssh_private_key`: *Optional.* An SSH key used to fetch modules, e.g. [private GitHub repos](https://www.terraform.io/docs/modules/sources.html#private-github-repos).

* `plan_only`: *Optional. Default `false`* This boolean will allow terraform to create a plan file and store it on S3. **Warning:** Plan files contain unencrypted credentials like AWS Secret Keys, only store these files in a private bucket.

* `plan_run`: *Optional. Default `false`* This boolean will allow terraform to execute the plan file stored on S3, then delete it.

* `import_files`: *Optional.* A list of files containing existing resources to [import](https://www.terraform.io/docs/import/usage.html) into the state file. The files can be in YAML or JSON format, containing key-value pairs like `aws_instance.bar: i-abcd1234`.

* `action`: *Optional.* Used to indicate a destructive `put`. The only recognized value is `destroy`, create / update are the implicit defaults.

  > **Note:** You must also set `put.get_params.action` to `destroy` to ensure the task succeeds. This is a temporary workaround until Concourse adds support for `delete` as a first-class operation. See [this issue](https://github.com/concourse/concourse/issues/362) for more details.

* `plugin_dir`: *Optional.* The path (relative to your `terraform_source`) of the directory containing plugin binaries. This overrides the default plugin directory and Terraform will not automatically fetch built-in plugins if this option is used. To preserve the automatic fetching of plugins, omit `plugin_dir` and place third-party plugins in `${terraform_source}/terraform.d/plugins`. See https://www.terraform.io/docs/configuration/providers.html#third-party-plugins for more information.

#### Put Example

```yaml
  - name: create-env-and-lock
    plan:
      # apply the terraform template with a random env_name
      - put: terraform
        params:
          generate_random_name: true
          delete_on_failure: true
          vars:
            subnet_cidr: 10.0.1.0/24
      # create a new pool-resource lock containing the terraform output
      - put: locks
        params:
          add: terraform/

  - name: destroy-env-and-lock
    plan:
      # acquire a lock
      - put: locks
        params:
          acquire: true
      # destroy the IaaS resources
      - put: terraform
        params:
          env_name_file: locks/name
          action: destroy
        get_params:
          action: destroy
      # destroy the lock
      - put: locks
        params:
          remove: locks/
```

#### Plan and apply example

```yaml
- name: terraform-plan
  plan:
    - put: terraform
      params:
        env_name: staging
        plan_only: true
        vars:
          subnet_cidr: 10.0.1.0/24

- name: terraform-apply
  plan:
    - get: terraform
      trigger: false
      passed: [terraform-plan]
    - put: terraform
      params:
        env_name: staging
        plan_run: true
        vars:
          subnet_cidr: 10.0.1.0/24
```

#### Metadata file

Every `put` action creates `name` and `metadata` files as an output containing the `env_name` and [Terraform Outputs](https://www.terraform.io/intro/getting-started/outputs.html) in JSON format.

```yaml
jobs:
  - put: terraform
    params:
      env_name: e2e
      terraform_source: project-src/terraform
  - task: show-outputs
    config:
      platform: linux
      inputs:
        - name: terraform
      run:
        path: /bin/sh
        args:
          - -c
          - |
              echo "name: $(cat terraform/name)"
              echo "metadata: $(cat terraform/metadata)"
```

The preceding job would show a file similar to the following:

```
name: e2e
metadata: { "vpc_id": "vpc-123456", "vpc_tag_name": "concourse" }
```

#### Examples

**Templates:**
- BOSH Director on [AWS with a single subnet](examples/aws/bosh-single-subnet.tf)

**Tasks:**
- [Generate director manifest](examples/tasks/director-manifest-aws.erb) from JSON file

See the [Concourse pipeline](ci/pipeline.yml) for additional examples.
