FROM golang:1.6-alpine

RUN apk update && \
   apk add ca-certificates unzip wget git bash

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

ENV TERRAFORM_VERSION=0.6.16

RUN \
   # Terraform 0.6.16 is dynamically linked and requires glibc, should be fixed in 0.6.17
   # Issue: https://github.com/hashicorp/terraform/issues/6998
   wget -q -O /etc/apk/keys/andyshinn.rsa.pub https://raw.githubusercontent.com/andyshinn/alpine-pkg-glibc/master/andyshinn.rsa.pub && \
   wget -O /tmp/glibc.apk https://github.com/andyshinn/alpine-pkg-glibc/releases/download/2.23-r1/glibc-2.23-r1.apk && \
   apk add /tmp/glibc.apk && \

   wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
   unzip /tmp/terraform.zip -d /usr/local/bin && \
   rm -rf /tmp/*
