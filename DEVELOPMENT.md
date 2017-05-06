## Development

#### Running tests

1. Create an S3 bucket for testing
1. Create an AWS user with the following permissions:

  ```json
  {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1476735201000",
            "Effect": "Allow",
            "NotAction": "s3:DeleteBucket",
            "Resource": [
                "arn:aws:s3:::YOUR_BUCKET",
                "arn:aws:s3:::YOUR_BUCKET/*"
            ]
        }
    ]
  }
  ```
1. `cp ./scripts/test.env.tpl ./tmp/test.env`
1. Fill in `./tmp/test.env` with your AWS creds
1. Run tests: `source ./tmp/test.env && ./scripts/run-tests`

#### Add / Updating dependencies

1. `cd ./src/terraform-resource`
1. `go get -u github.com/FiloSottile/gvt`
1. `gvt fetch -tag=v1.4.11 github.com/aws/aws-sdk-go`
1. `git add vendor/ && git commit`

#### Testing your changes in Concourse

1. Build a docker image with your changes:
  `./scripts/docker-build --image-name DOCKER_USER/IMAGE:TAG --terraform-url https://LATEST_TERRAFORM_URL.zip`
1. Include the image in your pipeline:
  ```yaml
  resource_types:
  - name: terraform
    type: docker-image
    source:
      repository: DOCKER_USER/IMAGE:TAG
  ```
1. Run your pipeline and verify everything works as expected.
1. Submit your changes as a PR!
