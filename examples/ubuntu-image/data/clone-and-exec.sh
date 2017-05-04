#!/bin/bash -e

if [ -z "$REPOSITORY" ]; then
  echo '$REPOSITORY must be a git repository'
  exit 1
fi

if [ -z "$REVISION" ]; then
  echo '$REVISION must be a revision or branch'
  exit 1
fi

mkdir -p '/src';
cd '/src';
git clone -q "$REPOSITORY" '/src';
git checkout "$REVISION";

exec "$@"
