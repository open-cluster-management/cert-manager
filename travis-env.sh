# Release Tag
if [ "$TRAVIS_BRANCH" = "master" ]; then
    RELEASE_TAG=latest
else
    if [ "$TRAVIS_TAG" != "" ]; then
        RELEASE_TAG="${TRAVIS_TAG#v}"
    else
        RELEASE_TAG="${TRAVIS_BRANCH#release-}-latest"
    fi
fi
export RELEASE_TAG="$RELEASE_TAG"

# Image version
if [ "$TRAVIS_IMAGE_VERSION" != ""] then
    IMAGE_VERSION = "${TRAVIS_IMAGE_VERSION}"
fi
export IMAGE_VERSION="$TRAVIS_IMAGE_VERSION"
# Release Tag
echo TRAVIS_EVENT_TYPE=$TRAVIS_EVENT_TYPE
echo TRAVIS_BRANCH=$TRAVIS_BRANCH
echo TRAVIS_TAG=$TRAVIS_TAG
echo RELEASE_TAG="$RELEASE_TAG"
