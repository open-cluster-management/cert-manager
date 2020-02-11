#!/bin/bash
set -e

export DOCKER_IMAGE_AND_TAG=${1}
export GOARCH=$(go env GOARCH)

for PROJECT in `ls cmd`; do
	export PROJECT
	echo "Begin building $PROJECT"
	make go-binary
	echo "Project: $PROJECT  cwd: $(pwd)"
	cp -v LICENSE hack/build/dockerfiles/$PROJECT
	cp -v License.txt hack/build/dockerfiles/$PROJECT
	cp -v packages.yaml hack/build/dockerfiles/$PROJECT
	export DOCKER_IMAGE=cert-manager-$PROJECT
	echo "Docker dir: $(ls hack/build/dockerfiles/$PROJECT)"
	make docker/build
	echo "Done building $PROJECT"
done
