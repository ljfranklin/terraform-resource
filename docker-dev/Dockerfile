FROM golang:1.6-alpine

RUN apk update && \
   apk add ca-certificates git bash

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

COPY terraform/* /usr/local/bin/
