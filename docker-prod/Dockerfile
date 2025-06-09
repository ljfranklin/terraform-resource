FROM alpine:latest

ARG TARGETARCH

RUN apk update && \
    apk add ca-certificates git bash openssh-client curl

# this glibc compatibility module is needed for some downloaded binaries,
# such as aws cli, to run in provisioners.
# Steps to build glibc for ARM64:
# https://gist.github.com/ljfranklin/f6a7dfcfb2c573d4a276dad59e58db82
# TODO: Switch back to upstream for ARM64 once supported:
# https://github.com/sgerrand/alpine-pkg-glibc/issues/126
# TODO: Remove --force-overwrite once
# https://github.com/sgerrand/alpine-pkg-glibc/issues/185 is fixed
RUN if [ "${TARGETARCH}" = "amd64" ]; then \
    wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
    wget -q -O /tmp/glibc.apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.32-r0/glibc-2.32-r0.apk && \
    wget -q -O /tmp/glibc-bin.apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.32-r0/glibc-bin-2.32-r0.apk; \
  elif [ "${TARGETARCH}" = "arm64" ]; then \
    wget -q -O /etc/apk/keys/ljfranklin-glibc.pub https://github.com/ljfranklin/alpine-pkg-glibc/releases/download/2.32-r0-arm64/ljfranklin-glibc.pub && \
    wget -q -O /tmp/glibc.apk https://github.com/ljfranklin/alpine-pkg-glibc/releases/download/2.32-r0-arm64/glibc-2.32-r0.apk && \
    wget -q -O /tmp/glibc-bin.apk https://github.com/ljfranklin/alpine-pkg-glibc/releases/download/2.32-r0-arm64/glibc-bin-2.32-r0.apk; \
  fi; \
  apk add --force-overwrite /tmp/glibc.apk && \
  apk add /tmp/glibc-bin.apk && \
  rm -rf /tmp/glibc.apk && \
  rm -rf /tmp/glibc-bin.apk

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

COPY $TARGETARCH/terraform/* /usr/local/bin/
RUN terraform --version
COPY $TARGETARCH/check $TARGETARCH/in $TARGETARCH/out /opt/resource/
