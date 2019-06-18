# Pushes images to integration if master and not a pull request.
if [ "$TRAVIS_BRANCH" = "master" ] && ! [ "$TRAVIS_EVENT_TYPE" = "pull_request" ]; then
    RETAG=true
    RELEASE_TAG=latest
    IMAGE_VERSION=0.7.0

    DOCKER_REGISTRY=hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com
    NAMESPACE=ibmcom
    #IMAGE_REPO=$(DOCKER_REGISTRY).artifactory.swg-devops.com

    IMAGE_VERSION_RHEL="${IMAGE_VERSION}-rhel"
    RELEASE_TAG_RHEL="${RELEASE_TAG}-rhel"

    export DOCKER_REGISTRY="$DOCKER_REGISTRY"
    export NAMESPACE="$NAMESPACE"
    export RELEASE_TAG="$RELEASE_TAG"
    export IMAGE_VERSION="$IMAGE_VERSION"
    export IMAGE_VERSION_RHEL="$IMAGE_VERSION_RHEL"
    export RELEASE_TAG_RHEL="$RELEASE_TAG_RHEL"
    export RETAG="$RETAG"
fi

echo TRAVIS_EVENT_TYPE=$TRAVIS_EVENT_TYPE
echo TRAVIS_BRANCH=$TRAVIS_BRANCH
