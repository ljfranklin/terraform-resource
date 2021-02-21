## Terraform Concourse Resource

A [Concourse](http://concourse-ci.org/) resource that allows jobs to modify IaaS resources via [Terraform](https://www.terraform.io/).
Useful for creating a pool of reproducible environments. No more snowflakes!

See [DEVELOPMENT](DEVELOPMENT.md) if you're interested in submitting a PR :+1:

![Docker Pulls](https://img.shields.io/docker/pulls/ljfranklin/terraform-resource.svg)

## Source Configuration

> **Important!:** The `source.storage` field has been replaced by `source.backend_type` and `source.backend_config` to leverage the built-in Terraform backends. If you currently use `source.storage` in your pipeline, follow the instructions in the [Backend Migration](#backend-migration) section to ensure your state files are not lost.

* `backend_type`: *Required.* The name of the [Terraform backend](https://www.terraform.io/docs/backends/types/index.html) the resource will use to store statefiles, e.g. `s3` or `consul`.

  > **Note:** The 'local' backend type is not supported, Concourse requires that state is persisted outside the container

* `backend_config`: *Required.* A map of key-value configuration options specific to your choosen backend, e.g. [S3 options](https://www.terraform.io/docs/backends/types/s3.html#configuration-variables).

* `env_name`: *Optional.* Name of the environment to manage, e.g. `staging`. A [Terraform workspace](https://www.terraform.io/docs/state/workspaces.html) will be created with this name. See [Single vs Pool](#managing-a-single-environment-vs-a-pool-of-environments) section below for more options.

* `delete_on_failure`: *Optional. Default `false`.* If true, the resource will run `terraform destroy` if `terraform apply` returns an error.

* `vars`: *Optional.* A collection of Terraform input variables.
These are typically used to specify credentials or override default module values.
See [Terraform Input Variables](https://www.terraform.io/intro/getting-started/variables.html) for more details.

* `env`: *Optional.* Similar to `vars`, this collection of key-value pairs can be used to pass environment variables to Terraform, e.g. "AWS_ACCESS_KEY_ID".

* `private_key`: *Optional.* An SSH key used to fetch modules, e.g. [private GitHub repos](https://www.terraform.io/docs/modules/sources.html#private-github-repos).

#### Source Example

```yaml
resource_types:
- name: terraform
  type: docker-image
  source:
    repository: ljfranklin/terraform-resource
    tag: latest

resources:
  - name: terraform
    type: terraform
    source:
      env_name: staging
      backend_type: s3
      backend_config:
        bucket: mybucket
        key: mydir/terraform.tfstate
        region: us-east-1
        access_key: {{storage_access_key}}
        secret_key: {{storage_secret_key}}
      vars:
        tag_name: concourse
      env:
        AWS_ACCESS_KEY_ID: {{environment_access_key}}
        AWS_SECRET_ACCESS_KEY: {{environment_secret_key}}
```

The above example uses AWS S3 to store Terraform state files. All `backend_config` options documented [here](https://www.terraform.io/docs/backends/types/s3.html#configuration-variables) are forwarded straight to Terraform.

Terraform also supports many other [state file backends](https://www.terraform.io/docs/backends/types/index.html), for example [Google Cloud Storage (GCS)](https://www.terraform.io/docs/backends/types/gcs.html):

```yaml
resources:
  - name: terraform
    type: terraform
    source:
      backend_type: gcs
      backend_config:
        bucket: mybucket
        prefix: mydir
        region: us-central1
        credentials: {{gcp_credentials_json}}
      ...
```

#### Image Variants

Note: all images support AMD64 and ARM64 architectures, although only AMD64 is fully tested prior to release.

- Latest stable release of resource: `ljfranklin/terraform-resource:latest`.
- Specific versions of Terraform, e.g. `ljfranklin/terraform-resource:0.7.7`.
- [RC builds](https://concourse.lylefranklin.com/teams/main/pipelines/terraform-resource-rc) from Terraform pre-releases: `ljfranklin/terraform-resource:rc`.
- [Nightly builds](https://concourse.lylefranklin.com/teams/main/pipelines/terraform-resource-nightly) from Terraform `master` branch: `ljfranklin/terraform-resource:nightly`.

See [Dockerhub](https://hub.docker.com/r/ljfranklin/terraform-resource/tags/) for a list of all available tags.
If you'd like to build your own image from a specific Terraform branch, configure a pipeline with [build-image-pipeline.yml](ci/build-image-pipeline.yml).

## Behavior

This resource should usually be used with the `put` action rather than a `get`.
This ensures the output always reflects the current state of the IaaS and allows management of multiple environments as shown below.
A `get` step outputs the same `metadata` file format shown below for `put`.

#### Get Parameters

> **Note:** In Concourse, a `put` is always followed by an implicit `get`. To pass `get` params via `put`, use `put.get_params`.

* `output_statefile`: *Optional. Default `false`* If true, the resource writes the Terraform statefile to a file named `terraform.tfstate`.**Warning:** Ensure any changes to this statefile are persisted back to the resource's storage bucket. **Another warning:** Some statefiles contain unencrypted secrets, be careful not to expose these in your build logs.
* `output_planfile`: *Optional. Default `false`* If true a file named `plan.json` with the JSON representation of the Terraform binary plan file will be created.   

* `output_module` *Optional.* Write only the outputs from the given module name to the `metadata` file.

#### Put Parameters

* `terraform_source`: *Required.* The relative path of the directory containing your Terraform configuration files.
For example: if your `.tf` files are stored in a git repo called `prod-config` under a directory `terraform-configs`, you could do a `get: prod-config` in your pipeline with `terraform_source: prod-config/terraform-configs/` as the source.

* `env_name`: *Optional, see Note.* The name of the environment to create or modify. A [Terraform workspace](https://www.terraform.io/docs/state/workspaces.html) will be created with this name. Multiple environments can be managed with a single resource.

* `generate_random_name`: *Optional, see Note. Default `false`* Generates a random `env_name` (e.g. "coffee-bee"). See [Single vs Pool](#managing-a-single-environment-vs-a-pool-of-environments) section below.

* `env_name_file`: *Optional, see Note.* Reads the `env_name` from a specified file path. Useful for destroying environments from a lock file.

  > Note: You must specify one of the following options: `source.env_name`, `put.params.env_name`, `put.params.generate_random_name`, or `env_name_file`

* `delete_on_failure`: *Optional. Default `false`.* See description under `source.delete_on_failure`.

* `vars`: *Optional.* A collection of Terraform input variables. See description under `source.vars`.

* `var_files`: *Optional.* A list of files containing Terraform input variables. These files can be in YAML, JSON, or HCL (filename must end in .tfvars) format.

  > Terraform variables will be merged from the following locations in increasing order of precedence: `source.vars`, `put.params.vars`, and `put.params.var_files`. Finally, `env_name` is automatically passed as an input `var`.

* `env`: *Optional.* A key-value collection of environment variables to pass to Terraform. See description under `source.env`.

* `private_key`: *Optional.* An SSH key used to fetch modules, e.g. [private GitHub repos](https://www.terraform.io/docs/modules/sources.html#private-github-repos).

* `plan_only`: *Optional. Default `false`* This boolean will allow Terraform to create a plan file and store it the configured backend. Useful for manually reviewing a plan prior to applying. See [Plan and Apply Example](#plan-and-apply-example). **Warning:** Plan files contain unencrypted credentials like AWS Secret Keys, only store these files in a private bucket.

* `plan_run`: *Optional. Default `false`* This boolean will allow Terraform to execute the plan file stored on the configured backend, then delete it.

* `import_files`: *Optional.* A list of files containing existing resources to [import](https://www.terraform.io/docs/import/usage.html) into the state file. The files can be in YAML or JSON format, containing key-value pairs like `aws_instance.bar: i-abcd1234`.

* `override_files`: *Optional.* A list of files to copy into the `terraform_source` directory. Override files must follow conventions outlined [here](https://www.terraform.io/docs/configuration/override.html) such as file names ending in `_override.tf`.

* `module_override_files`: *Optional.* A list of maps to copy override files to specific destination directories. Override files must follow conventions outlined [here](https://www.terraform.io/docs/configuration/override.html) such as file names ending in `_override.tf`.
The source file is specified with `src` and the destination directory with `dst`. 

* `action`: *Optional.* When set to `destroy`, the resource will run `terraform destroy` against the given statefile.
  > **Note:** You must also set `put.get_params.action` to `destroy` to ensure the task succeeds. This is a temporary workaround until Concourse adds support for `delete` as a first-class operation. See [this issue](https://github.com/concourse/concourse/issues/362) for more details.

* `plugin_dir`: *Optional.* The path (relative to your `terraform_source`) of the directory containing plugin binaries. This overrides the default plugin directory and Terraform will not automatically fetch built-in plugins if this option is used. To preserve the automatic fetching of plugins, omit `plugin_dir` and place third-party plugins in `${terraform_source}/terraform.d/plugins`. See https://www.terraform.io/docs/configuration/providers.html#third-party-plugins for more information.

#### Put Example

Every `put` action creates `name` and `metadata` files as an output containing the `env_name` and [Terraform Outputs](https://www.terraform.io/intro/getting-started/outputs.html) in JSON format.

```yaml
jobs:
- name: update-infrastructure
  plan:
  - get: project-git-repo
  - put: terraform
    params:
      env_name: e2e
      terraform_source: project-git-repo/terraform
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

#### Plan and apply example

```yaml
jobs:
- name: terraform-plan
  plan:
  - get: project-git-repo
  - put: terraform
    params:
      env_name: staging
      terraform_source: project-git-repo/terraform
      plan_only: true
      vars:
        subnet_cidr: 10.0.1.0/24

- name: terraform-apply
  plan:
  - get: project-git-repo
    trigger: false
    passed: [terraform-plan]
  - get: terraform
    trigger: false
    passed: [terraform-plan]
  - put: terraform
    params:
      env_name: staging
      terraform_source: project-git-repo/terraform
      plan_run: true
```

## Managing a single environment vs a pool of environments

This resource can be used to manage a single environment or a pool of many environments.

#### Single Environment

To use this resource to manage a single environment, set `source.env_name` or `put.params.env_name` to a fixed name like `staging` or `production` as shown in the previous `put` example.
Each `put` will update the IaaS resources and state file for that environment.

#### Pool of Environments

To manage a pool of many environments, you can use this resource in combination with the [pool-resource](https://github.com/concourse/pool-resource).
This allows you to create a pool of identical environments that can be claimed and released by CI jobs and humans.
Setting `put.params.generate_random_name: true` will create a random, unique `env_name` like "coffee-bee" for each environment, and
the pool-resource will persist the name and metadata for these environments in a private `git` repo.

```yaml
jobs:
- name: create-env-and-lock
  plan:
    # apply the terraform template with a random env_name
    - get: project-git-repo
    - put: terraform
      params:
        terraform_source: project-git-repo/terraform
        generate_random_name: true
        delete_on_failure: true
        vars:
          subnet_cidr: 10.0.1.0/24
    # create a new pool-resource lock containing the terraform output
    - put: locks
      params:
        add: terraform/

- name: claim-env-and-test
  plan:
    # claim a random env lock
    - put: locks
      params:
        acquire: true
    # the locks dir will contain `name` and `metadata` files described above
    - task: run-tests-against-env
      file: test.yml
      input_mapping:
        env: locks/

- name: destroy-env-and-lock
  plan:
    - get: project-git-repo
    # acquire a lock
    - put: locks
      params:
        acquire: true
    # destroy the IaaS resources
    - put: terraform
      params:
        terraform_source: project-git-repo/terraform
        env_name_file: locks/name
        action: destroy
      get_params:
        action: destroy
    # destroy the lock
    - put: locks
      params:
        remove: locks/
```

## Backend Migration

Previous versions of this resource required statefiles to be stored in an S3-compatible blobstore using the `source.storage` field.
The latest version of this resource instead uses the build-in [Terraform Backends](https://www.terraform.io/docs/backends/types/index.html) to support many other statefile storage options in addition to S3.
If you have an existing pipeline that uses `source.storage`, your statefiles will need to be migrated into the new backend directory structure using the following steps:

1. Rename `source.storage` to `source.migrated_from_storage` in your pipeline config. All fields within `source.storage` should remain unchanged, only the top-level key should be renamed.
2. Add `source.backend_type` and `source.backend_config` fields as described under [Source Configuration](#source-configuration).
3. Update your pipeline: `fly set-pipeline`.
4. The next time your pipeline performs a `put` to the Terraform resource:
  - The resource will copy the statefile for the modified environment into the new directory structure.
  - The resource will rename the old statefile in S3 to `$ENV_NAME.migrated`.
5. Once all statefiles have been migrated and everything is working as expected, you may:
  - Remove the old `.migrated` statefiles.
  - Remove the `source.migrated_from_storage` from your pipeline config.

> Breaking Change: The backend mode drops support for feeding Terraform outputs back in as input vars to subsequent puts. This "feature" causes suprising errors if inputs and outputs have the same name but different types and the implementation was significantly more complicated with the new migrated_from_storage flow.

#### Legacy storage configuration

* `migrated_from_storage.bucket`: *Required.* The S3 bucket used to store the state files.

* `migrated_from_storage.bucket_path`: *Required.* The S3 path used to store state files, e.g. `mydir/`.

* `migrated_from_storage.access_key_id`: *Required.* The AWS access key used to access the bucket.

* `migrated_from_storage.secret_access_key`: *Required.* The AWS secret key used to access the bucket.

* `migrated_from_storage.region_name`: *Optional.* The AWS region where the bucket is located.

* `migrated_from_storage.server_side_encryption`: *Optional.* An encryption algorithm to use when storing objects in S3, e.g. "AES256".

* `migrated_from_storage.sse_kms_key_id` *Optional.* The ID of the AWS KMS master encryption key used for the object.

* `migrated_from_storage.endpoint`: *Optional.* The endpoint for an s3-compatible blobstore (e.g. Ceph).

  > **Note:** By default, the resource will use S3 signing version v2 if an endpoint is specified as many non-S3 blobstores do not support v4.
Opt into v4 signing by setting `migrated_from_storage.use_signing_v4: true`.

#### Migration Example

```yaml
resources:
  - name: terraform
    type: terraform
    source:
      backend_type: s3
      backend_config:
        bucket: mybucket
        key: mydir/terraform.tfstate
        region: us-east-1
        access_key: {{storage_access_key}}
        secret_key: {{storage_secret_key}}
      migrated_from_storage:
        bucket: mybucket
        bucket_path: mydir/
        region: us-east-1
        access_key_id: {{storage_access_key}}
        secret_access_key: {{storage_secret_key}}
      vars:
        tag_name: concourse
      env:
        AWS_ACCESS_KEY_ID: {{environment_access_key}}
        AWS_SECRET_ACCESS_KEY: {{environment_secret_key}}
```
