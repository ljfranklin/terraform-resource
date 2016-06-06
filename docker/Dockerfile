FROM alpine:latest

# Terraform 0.6.16 is dynamically linked and requires glibc, should be fixed in 0.6.17
# Issue: https://github.com/hashicorp/terraform/issues/6998
RUN apk update && \
    apk add ca-certificates wget && \
    wget -q -O /etc/apk/keys/andyshinn.rsa.pub https://raw.githubusercontent.com/andyshinn/alpine-pkg-glibc/master/andyshinn.rsa.pub && \
    wget -O /tmp/glibc.apk https://github.com/andyshinn/alpine-pkg-glibc/releases/download/2.23-r1/glibc-2.23-r1.apk && \
    apk add /tmp/glibc.apk && \
    rm -rf /tmp/glibc.apk

WORKDIR ../../build-artifacts

COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
