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
              "Action": [
                  "s3:DeleteObject",
                  "s3:GetObject",
                  "s3:ListBucket",
                  "s3:PutObject"
              ],
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
1. `gvt {fetch,update} github.com/your/dep`
1. `git add vendor/ && git commit`
