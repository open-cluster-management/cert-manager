# Copyright Contributors to the Open Cluster Management project

#!/bin/bash
set -e

export ICP=true
export DOCKER_IMAGE_AND_TAG=${1}
# Go verify
make go-verify
make go/gosec-install
make go-coverage
