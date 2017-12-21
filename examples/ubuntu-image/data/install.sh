#!/bin/bash

echo ' - Removing password from 'worker' user'
passwd -d worker

echo ' - Installing taskcluster-worker qemu-guest-tools'
cp /tmp/custom-data/taskcluster-worker /usr/local/bin/taskcluster-worker
chmod +x /usr/local/bin/taskcluster-worker
cp /tmp/custom-data/taskcluster-worker.service /etc/systemd/system/taskcluster-worker.service
chmod 644 /etc/systemd/system/taskcluster-worker.service
systemctl enable taskcluster-worker.service

echo ' - Installing docker'
apt-get install -y apt-transport-https ca-certificates
apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
echo 'deb https://apt.dockerproject.org/repo ubuntu-xenial main' > /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-engine

echo ' - Installing build dependencies'
apt-get install -y git curl screen nano build-essential

echo ' - Installing test script'
cp '/tmp/custom-data/clone-and-exec.sh' /usr/local/bin/clone-and-exec.sh;
chmod +x /usr/local/bin/clone-and-exec.sh;
