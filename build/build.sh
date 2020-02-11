#!/bin/bash
set -e

export GOARCH=$(go env GOARCH)

make docker/login
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
	make docker/push
	echo "Done building $PROJECT"
done
