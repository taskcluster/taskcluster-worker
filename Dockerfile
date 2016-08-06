FROM ubuntu:16.04
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

RUN apt-get update -y
RUN apt-get upgrade -y

# Install runtime dependencies
RUN apt-get install -y qemu-system-x86 qemu-utils dnsmasq-base liblz4-tool iptables

# Install build dependencies
RUN apt-get install -y golang git curl screen nano genisoimage build-essential

ENV APP_PATH    github.com/taskcluster/taskcluster-worker
ENV GOPATH      /go
ENV SHELL       bash

RUN mkdir -p /go/src/$APP_PATH
RUN ln -s /go/src/$APP_PATH /src

RUN go get github.com/smartystreets/goconvey

WORKDIR /go/src/$APP_PATH
