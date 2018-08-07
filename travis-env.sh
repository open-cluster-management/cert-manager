# Release Tag
if [ "$TRAVIS_BRANCH" = "master" ]; then
    RELEASE_TAG=latest
    ARTIFACTORY_IMAGE_REPO=hyc-cloud-private-integration-docker-local
    ARTIFACTORY_NAMESPACE=ibmcom
    ARTIFACTORY_RELEASE_TAG=0.3.0
    if [ "$OS" = "rhel7" ]; then
        ARTIFACTORY_RELEASE_TAG="${ARTIFACTORY_RELEASE_TAG}-rhel"
    fi
    export ARTIFACTORY_IMAGE_REPO="$ARTIFACTORY_IMAGE_REPO"
    export ARTIFACTORY_NAMESPACE="$ARTIFACTORY_NAMESPACE"
    export ARTIFACTORY_RELEASE_TAG="$ARTIFACTORY_RELEASE_TAG"
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
