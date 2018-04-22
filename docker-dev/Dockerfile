FROM golang:1.9-alpine

RUN apk update && \
   apk --no-cache add ca-certificates git bash wget gnupg unzip make \
                      libc6-compat openssh-client

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

COPY terraform/* /usr/local/bin/
