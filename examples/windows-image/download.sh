#!/bin/bash

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

# Ensure cache dir exists
mkdir -p "$DIR/cache";

# Download Windows ISO
if ! [ -f "$INSTALLER_ISO" ]; then
  if [ -z "$AWS_ACCESS_KEY_ID" ]; then
    echo 'Fetching S3 credentials'
    # Get S3 credentials through auth.taskcluster.net using tcproxy (only works from a task)
    # requires task.scopes: 'auth:aws-s3:read-only:private-qemu-images/*'
    curl -L --retry 10 "http://taskcluster/tcproxy/auth.taskcluster.net/v1/aws/s3/read-only/$INSTALLER_S3_BUCKET/$INSTALLER_S3_KEY" > /tmp/s3-credentials.json
    export AWS_ACCESS_KEY_ID=`cat /tmp/s3-credentials.json | jq -r .credentials.accessKeyId`
    export AWS_SECRET_ACCESS_KEY=`cat /tmp/s3-credentials.json | jq -r .credentials.secretAccessKey`
    export AWS_SESSION_TOKEN=`cat /tmp/s3-credentials.json | jq -r .credentials.sessionToken`
    rm -rf /tmp/s3-credentials.json
  fi

  echo 'Downloading Windows ISO'
  aws s3 cp "s3://$INSTALLER_S3_BUCKET/$INSTALLER_S3_KEY" "$INSTALLER_ISO"
fi

# Download virtio ISO
if ! [ -f "$VIRTIO_WIN_ISO" ]; then
  echo 'Downloading virtio ISO'
  curl -L --retry 10 "$VIRTIO_WIN_URL" -o "$VIRTIO_WIN_ISO"
fi
