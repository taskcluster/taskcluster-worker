FROM ubuntu:17.04
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

# Install dependencies
RUN apt-get update -y \
 && apt-get upgrade -y \
 && apt-get install -y \
    qemu-system-x86 qemu-utils dnsmasq-base iptables iproute2 \
    git curl screen nano genisoimage build-essential openvpn s3cmd jq

# Install golang 1.8
RUN curl -L https://storage.googleapis.com/golang/go1.8.linux-amd64.tar.gz > /tmp/go.tar.gz \
 && tar -xvf /tmp/go.tar.gz -C /usr/local \
 && rm /tmp/go.tar.gz

# Install zstd 1.1.4
RUN curl -L https://github.com/facebook/zstd/archive/v1.1.4.tar.gz > /tmp/zstd.tar.gz \
 && tar -xvf /tmp/zstd.tar.gz -C /tmp \
 && make -j -C /tmp/zstd-1.1.4/programs install \
 && rm -rf /tmp/zstd-1.1.4/ /tmp/zstd.tar.gz

RUN mkdir -p /go

ENV GOPATH      /go
ENV PATH        /usr/local/go/bin:$GOPATH/bin:$PATH
ENV APP_PATH    github.com/taskcluster/taskcluster-worker
ENV SHELL       bash

# Install govendor
RUN go get github.com/kardianos/govendor

RUN mkdir -p /go/src/$APP_PATH

WORKDIR /go/src/$APP_PATH
