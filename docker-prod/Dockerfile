FROM alpine:latest

RUN apk update && \
    apk add ca-certificates git bash libc6-compat openssh-client

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
