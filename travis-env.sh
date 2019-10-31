# Pushes images to integration if master and not a pull request.
if [ "$TRAVIS_BRANCH" = "master" ] && ! [ "$TRAVIS_EVENT_TYPE" = "pull_request" ]; then
    RETAG=true
    DOCKER_REGISTRY=hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com
    NAMESPACE=ibmcom

    export DOCKER_REGISTRY="$DOCKER_REGISTRY"
    export NAMESPACE="$NAMESPACE"
    export RETAG="$RETAG"
fi

echo TRAVIS_EVENT_TYPE=$TRAVIS_EVENT_TYPE
echo TRAVIS_BRANCH=$TRAVIS_BRANCH
