#!/bin/sh

set -o errexit

godep go build github.com/GoogleCloudPlatform/kubernetes/cmd/provisioner

docker build -t vish/provisioner:test .

docker push vish/provisioner:test
