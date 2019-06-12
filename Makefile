include Configfile
# Copyright 2019 The Jetstack cert-manager contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# CICD BUILD HARNESS
####################
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')

.PHONY: default
default:: init;

.PHONY: init\:
init::
	@mkdir -p variables
ifndef GITHUB_USER
	$(info GITHUB_USER not defined)
	exit -1
endif
	$(info Using GITHUB_USER=$(GITHUB_USER))
ifndef GITHUB_TOKEN
	$(info GITHUB_TOKEN not defined)
	exit -1
endif

-include $(shell curl -fso .build-harness -H "Authorization: token ${GITHUB_TOKEN}" -H "Accept: application/vnd.github.v3.raw" "https://raw.github.ibm.com/ICP-DevOps/build-harness/master/templates/Makefile.build-harness"; echo .build-harness)
####################
# Look at what these mean
GOLDFLAGS := -ldflags "-X $(PACKAGE_NAME)/pkg/util.AppGitState=${GIT_STATE} -X $(PACKAGE_NAME)/pkg/util.AppGitCommit=${GIT_COMMIT} -X $(PACKAGE_NAME)/pkg/util.AppVersion=${APP_VERSION}"

.PHONY: lint build go-binary docker-image docker-push rhel-image \
	go-test go-fmt go-verify

# Docker build flags
DOCKER_BUILD_FLAGS := --build-arg VCS_REF=$(GIT_COMMIT) $(DOCKER_BUILD_FLAGS)

lint:
	@git diff-tree --check $(shell git hash-object -t tree /dev/null) HEAD $(shell ls -d * | grep -v vendor | grep -v docs | grep -v deploy | grep -v hack)

# Alias targets
###############
image:: build
build: go-binary docker-image

# Go targets
#################
go-verify: go-fmt go-test

# Builds the go binaries for the project.
go-binary:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-a -tags netgo \
		-o $(DOCKERFILES)/$(PROJECT)/${APP_NAME}-$(PROJECT)_$(GOOS)_$(GOARCH) \
		$(GOLDFLAGS) \
		./cmd/$(PROJECT)

go-test:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure
	go test -v \
		$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/client' | \
			grep -v '/third_party' | \
			grep -v '/docs/' \
		)

go-fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | grep -v 'third_party/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

# Docker targets
################
docker-image:
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME := $(APP_NAME)-$(PROJECT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)-$(ARCH))
	$(eval IMAGE := $(DOCKER_REGISTRY)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))
	$(eval DOCKER_FILE := Dockerfile$(DOCKER_FILE_EXT))
	$(eval DOCKERFILE_PATH := $(DOCKERFILES)/$(PROJECT))
	@echo "PROJECT: $(PROJECT) and PATH: $(DOCKERFILE_PATH)"
	@echo "App: $(IMAGE_NAME_ARCH):$(IMAGE_VERSION)"

	@echo "HOME: $(TRAVIS_BUILD_DIR)"
	cp /home/travis/gopath/src/github.com/jetstack/cert-manager/LICENSE $(DOCKERFILE_PATH)
	cp /home/travis/gopath/src/github.com/jetstack/cert-manager/License.txt $(DOCKERFILE_PATH)
	cp /home/travis/gopath/src/github.com/jetstack/cert-manager/packages.yaml $(DOCKERFILE_PATH)

	$(eval DOCKER_BUILD_OPTS := '--build-arg "VCS_REF=$(GIT_COMMIT)" \
           --build-arg "VCS_URL=$(GIT_REMOTE_URL)" \
           --build-arg "IMAGE_NAME=$(IMAGE_NAME_ARCH)" \
           --build-arg "IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION)" \
		   --build-arg "SUMMARY=$(SUMMARY)" \
		   --build-arg "GOARCH=$(GOARCH)"')

	# Building docker image.
	@make DOCKER_BUILD_PATH=$(DOCKERFILE_PATH) \
			DOCKER_BUILD_OPTS=$(DOCKER_BUILD_OPTS) \
			DOCKER_IMAGE=$(IMAGE) \
			DOCKER_BUILD_TAG=$(IMAGE_VERSION) \
			DOCKER_FILE=$(DOCKER_FILE) docker:build
	@echo "Built docker image."

docker-push:
	$(eval IMAGE_NAME := $(APP_NAME)-$(PROJECT))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)-$(ARCH))
	$(eval IMAGE := $(DOCKER_REGISTRY)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))
	$(eval DOCKER_URI := $(IMAGE):$(IMAGE_VERSION))
	# Pushing docker image.
	@make DOCKER_URI=$(DOCKER_URI) docker:push
	@echo "Pushed $(IMAGE):$(IMAGE_VERSION) to $(IMAGE)"

ifneq ($(RETAG),)
	$(eval DOCKER_URI := $(IMAGE):$(RELEASE_TAG))
	@make DOCKER_IMAGE=$(IMAGE) \
		DOCKER_BUILD_TAG=$(IMAGE_VERSION) \
		DOCKER_URI=$(DOCKER_URI) \
		docker:tag
	@make DOCKER_URI=$(DOCKER_URI) docker:push
	@echo "Retagged image as $(IMAGE):$(RELEASE_TAG) and pushed to $(IMAGE)"
endif

# Retags the image with the rhel tag.
rhel-image:
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_RETAG_CMD))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)-$(ARCH))
	$(eval IMAGE := $(DOCKER_REGISTRY)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))
	$(eval IMAGE_VERSION_RHEL ?= $(APP_VERSION)-$(GIT_COMMIT)-rhel)
	$(eval IMAGE_RETAG := $(IMAGE):$(IMAGE_VERSION_RHEL))

	@make DOCKER_IMAGE=$(IMAGE) \
		DOCKER_BUILD_TAG=$(IMAGE_VERSION) \
		DOCKER_URI=$(IMAGE_RETAG)
		docker:tag
	@make DOCKER_URI=$(IMAGE_RETAG) docker:push
	@echo "Retagged image as $(IMAGE):$(IMAGE_VERSION_RHEL) and pushed to $(REPO_URL)"

ifneq ($(RETAG),)
	$(eval IMAGE_RETAG := $(IMAGE):$(RELEASE_TAG_RHEL))
	@make DOCKER_IMAGE=$(IMAGE) \
		DOCKER_BUILD_TAG=$(IMAGE_VERSION_RHEL) \
		DOCKER_URI=$(IMAGE_RETAG)
		docker:tag
	@make DOCKER_URI=$(IMAGE_RETAG) docker:push
	@echo "Retagged image as $(IMAGE):$(RELEASE_TAG_RHEL) and pushed to $(REPO_URL)"
endif
