#!/bin/sh

set -o errexit

godep go build github.com/GoogleCloudPlatform/kubernetes/cmd/scaler

docker build -t vish/scaler:test .

docker push vish/scaler:test
