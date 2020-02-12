#!/bin/bash
set -e

export ICP=true
export DOCKER_IMAGE_AND_TAG=${1}
make go-verify
