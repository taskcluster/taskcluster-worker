#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo '### Installing taskcluster-worker qemu-guest-tools'
cp "$DIR"/taskcluster-worker /usr/local/bin/
chmod +x /usr/local/bin/taskcluster-worker

echo '### Update packages'
apt-get update -y
apt-get upgrade -y

echo '### Installing docker'
apt-get install -y apt-transport-https ca-certificates
apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
echo 'deb https://apt.dockerproject.org/repo ubuntu-xenial main' > /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-engine

echo '### Installing build dependencies'
apt-get install -y git curl screen nano build-essential

echo '### Installing test script'
cp "$DIR/clone-and-exec.sh" /usr/local/bin/clone-and-exec.sh;
chmod +x /usr/local/bin/clone-and-exec.sh;

echo '### Installing systemd service'
cp "$DIR/taskcluster-worker.service" /etc/systemd/system/
chmod 644 /etc/systemd/system/taskcluster-worker.service
systemctl enable taskcluster-worker.service

echo '### Setup done'
