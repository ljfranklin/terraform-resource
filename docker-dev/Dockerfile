FROM golang:1.6-alpine

RUN apk update && \
   apk --no-cache add ca-certificates git bash wget unzip

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

COPY terraform/* /usr/local/bin/
