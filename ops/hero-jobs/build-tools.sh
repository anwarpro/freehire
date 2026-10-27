#!/bin/sh
set -eu
cd /opt/freehire
docker build --target build -t hero-jobs-build:local .
cd /opt/hero-jobs
docker build -f Dockerfile.tools -t hero-jobs-tools:local .
