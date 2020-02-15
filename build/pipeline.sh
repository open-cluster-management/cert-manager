#!/bin/bash
set -e

echo "Pipeline for certificate manager starting : $(date)"
for PROJECT in `ls cmd`; do
	export PROJECT
	echo "Begin pipeline for cert-manager $PROJECT"
	export DOCKER_IMAGE=cert-manager-$PROJECT
	export COMPONENT_NAME=$(cat COMPONENT_NAME 2> /dev/null)-$PROJECT
	export COMPONENT_VERSION=$(cat COMPONENT_VERSION 2> /dev/null)
	export COMPONENT_DOCKER_REPO="quay.io/open-cluster-management"
	export DOCKER_IMAGE_AND_TAG=${COMPONENT_DOCKER_REPO}/${COMPONENT_NAME}:${COMPONENT_VERSION}${COMPONENT_TAG_EXTENSION}
	make pipeline-manifest/update PIPELINE_MANIFEST_COMPONENT_SHA256=${TRAVIS_COMMIT} PIPELINE_MANIFEST_COMPONENT_REPO=${TRAVIS_REPO_SLUG} PIPELINE_MANIFEST_BRANCH=${TRAVIS_BRANCH}
	echo "Completed pipeline for cert-manager $PROJECT"
done
echo "Pipeline for certificate manager completed : $(date)"
