#!/bin/bash
set -e

export GOARCH=$(go env GOARCH)
export SAVED_COMPONENT=$(cat COMPONENT_NAME 2> /dev/null)
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
	export COMPONENT_NAME=$(cat COMPONENT_NAME 2> /dev/null)-$PROJECT
	export COMPONENT_VERSION=$(cat COMPONENT_VERSION 2> /dev/null)
	export COMPONENT_DOCKER_REPO="quay.io/open-cluster-management"
	export DOCKER_IMAGE_AND_TAG=${COMPONENT_DOCKER_REPO}/${COMPONENT_NAME}:${COMPONENT_VERSION}${COMPONENT_TAG_EXTENSION}
	make docker/build
	if [ `go env GOOS` == "linux" ]; then
		make component/push
	fi

	# Security scans read the image from the COMPONENT_NAME file
	echo "$COMPONENT_NAME" > COMPONENT_NAME
        make security/scans
	# Undo changes
	echo "$SAVED_COMPONENT" > COMPONENT_NAME
	export COMPONENT_NAME=$(cat COMPONENT_NAME)

	echo "Done building $PROJECT"
done
echo "Building certificate manager completed : $(date)"
