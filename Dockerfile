FROM golang:1.6

COPY . /go/src/github.com/ljfranklin/terraform-resource
WORKDIR /go/src/github.com/ljfranklin/terraform-resource

RUN go build -o /opt/resource/check ./check/ && \
    go build -o /opt/resource/in ./in/ && \
    go build -o /opt/resource/out ./out/
