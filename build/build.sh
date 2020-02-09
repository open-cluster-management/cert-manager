#!/bin/bash
set -e

export DOCKER_IMAGE_AND_TAG=${1}
export GOARCH=$(go env GOARCH)

for PROJECT in `ls hack/build/dockerfiles`; do
	export PROJECT
	make go-binary
	export DOCKER_IMAGE=cert-manager-$PROJECT
	make docker/build
done
