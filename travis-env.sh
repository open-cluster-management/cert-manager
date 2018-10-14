# Pushes images to integration if master and not a pull request.
if [ "$TRAVIS_BRANCH" = "master" ] && ! [ "$TRAVIS_EVENT_TYPE" = "pull_request" ]; then
    RETAG=true
    RELEASE_TAG=latest
    IMAGE_VERSION=0.5.0
    
    IMAGE_REPO=hyc-cloud-private-integration-docker-local
    NAMESPACE=ibmcom

    if [ "$OS" = "rhel7" ]; then
        IMAGE_VERSION="${IMAGE_VERSION}-rhel"
        RELEASE_TAG="${RELEASE_TAG}-rhel"
    fi

    export IMAGE_REPO="$IMAGE_REPO"
    export NAMESPACE="$NAMESPACE"
    export RELEASE_TAG="$RELEASE_TAG"
    export IMAGE_VERSION="$IMAGE_VERSION"
    export RETAG="$RETAG"
fi

echo TRAVIS_EVENT_TYPE=$TRAVIS_EVENT_TYPE
echo TRAVIS_BRANCH=$TRAVIS_BRANCH
