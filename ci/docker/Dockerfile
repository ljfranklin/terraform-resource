FROM golang:1.6-alpine

RUN apk update && \
   apk add ca-certificates unzip wget git bash

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

ENV TERRAFORM_VERSION=0.7.0

RUN \
   wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
   unzip /tmp/terraform.zip -d /usr/local/bin && \
   rm -rf /tmp/*
