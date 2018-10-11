# Release Tag
if [ "$TRAVIS_BRANCH" = "master" ] && ! [ "$TRAVIS_EVENT_TYPE" = "pull_request" ]; then
    RETAG=true
    RELEASE_TAG=0.5.0
    IMAGE_VERSION=0.5.0
    
    IMAGE_REPO=hyc-cloud-private-integration-docker-local
    NAMESPACE=ibmcom
    ARTIFACTORY_RELEASE_TAG=latest

    if [ "$OS" = "rhel7" ]; then
        ARTIFACTORY_RELEASE_TAG="${ARTIFACTORY_RELEASE_TAG}-rhel"
    fi

    export IMAGE_REPO="$IMAGE_REPO"
    export NAMESPACE="$NAMESPACE"
    export ARTIFACTORY_RELEASE_TAG="$ARTIFACTORY_RELEASE_TAG"
    export IMAGE_VERSION="$IMAGE_VERSION"
    export RETAG="$RETAG"
else
    if [ "$TRAVIS_TAG" != "" ]; then
        RELEASE_TAG="${TRAVIS_TAG#v}"
    else
        RELEASE_TAG="${TRAVIS_BRANCH#release-}-latest"
    fi

    # Image version
    if [ "$TRAVIS_IMAGE_VERSION" != ""]; then
        IMAGE_VERSION="${TRAVIS_IMAGE_VERSION}"
        export IMAGE_VERSION="$TRAVIS_IMAGE_VERSION"
    fi

fi
export RELEASE_TAG="$RELEASE_TAG"

# Release Tag
echo TRAVIS_EVENT_TYPE=$TRAVIS_EVENT_TYPE
echo TRAVIS_BRANCH=$TRAVIS_BRANCH
echo TRAVIS_TAG=$TRAVIS_TAG
echo RELEASE_TAG="$RELEASE_TAG"
