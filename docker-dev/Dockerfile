FROM golang:alpine

# Temporarily work around missing bzr in v3.12 branch
RUN echo '@previous http://nl.alpinelinux.org/alpine/v3.11/community' >> /etc/apk/repositories
RUN apk update && \
   apk --no-cache add ca-certificates git bash wget gnupg unzip make \
                      openssh-client build-base bzr@previous

RUN wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
  wget -q -O /tmp/glibc.apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.32-r0/glibc-2.32-r0.apk && \
  apk add /tmp/glibc.apk && \
  rm -rf /tmp/glibc.apk

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

COPY terraform/* /usr/local/bin/
