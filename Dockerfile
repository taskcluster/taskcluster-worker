FROM golang:1.6
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

RUN apt-get update -y
RUN apt-get upgrade -y
RUN apt-get install -y qemu dnsmasq-base build-essential liblz4-tool iptables

ENV APP_PATH    github.com/taskcluster/taskcluster-worker

RUN mkdir -p /go/src/$APP_PATH
RUN ln -s /go/src/$APP_PATH /src
WORKDIR /go/src/$APP_PATH
