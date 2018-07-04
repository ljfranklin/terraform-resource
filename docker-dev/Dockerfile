FROM golang:alpine

RUN apk update && \
   apk --no-cache add ca-certificates git bash wget gnupg unzip make \
                      libc6-compat openssh-client

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

COPY terraform/* /usr/local/bin/
