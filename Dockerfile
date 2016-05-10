FROM golang:1.6

# install apt deps
RUN apt-get -y update && \
    apt-get -y install unzip && \
    apt-get clean

# install terraform
RUN wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/0.6.16/terraform_0.6.16_linux_amd64.zip && \
    unzip /tmp/terraform.zip -d /usr/local/bin/ && \
    rm /tmp/terraform.zip

# build resource
COPY . /go/src/github.com/ljfranklin/terraform-resource
WORKDIR /go/src/github.com/ljfranklin/terraform-resource

RUN go build -o /opt/resource/check ./cmd/check/ && \
    go build -o /opt/resource/in ./cmd/in/ && \
    go build -o /opt/resource/out ./cmd/out/
