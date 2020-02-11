#!/bin/bash
set -e

export GOARCH=$(go env GOARCH)
echo "Building certificate manager starting : $(date)"
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
	export COMPONENT_NAME=$(cat COMPONENT_NAME)-$PROJECT
	make component/push
	export COMPONENT_NAME=$(cat COMPONENT_NAME)
	echo "Done building $PROJECT"
done
echo "Building certificate manager completed : $(date)"
