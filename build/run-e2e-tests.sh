#!/bin/bash
set -e

export DOCKER_IMAGE_AND_TAG=${1}
# create kind cluster
# deploy the cert-manager controller
# pull down the cert-manager-test-automation repo
# cd to that repo
# make test:adoption
