include Configfile

# AppVersion is set as the AppVersion to be compiled into the controller binary.
# It's used as the default version of the 'acmesolver' image to use for ACME
# challenge requests, and any other future provider that requires additional
# image dependencies will use this same tag.
ifeq ($(APP_VERSION),)
APP_VERSION := $(if $(shell cat VERSION 2> /dev/null),$(shell cat VERSION 2> /dev/null),0.3.0)
endif

# Get a list of all binaries to be built
CMDS := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)
# Path to dockerfiles directory
DOCKERFILES := $(HACK_DIR)/build/dockerfiles
# A list of all types.go files in pkg/apis
TYPES_FILES := $(shell find pkg/apis -name types.go)
# docker_build_controller, docker_build_apiserver etc
DOCKER_BUILD_TARGETS := $(addprefix docker_build_, $(CMDS))
# docker_push_controller, docker_push_apiserver etc
DOCKER_PUSH_TARGETS := $(addprefix docker_push_, $(CMDS))
# docker_push_controller, docker_push_apiserver etc
DOCKER_RELEASE_TARGETS := $(addprefix docker_release_, $(CMDS))

# Go build flags
GOOS := linux

GIT_COMMIT := $(shell git rev-parse HEAD)
GOLDFLAGS := -ldflags "-X $(PACKAGE_NAME)/pkg/util.AppGitState=${GIT_STATE} -X $(PACKAGE_NAME)/pkg/util.AppGitCommit=${GIT_COMMIT} -X $(PACKAGE_NAME)/pkg/util.AppVersion=${APP_VERSION}"

.PHONY: verify build docker_build push generate generate_verify deploy_verify artifactory_login \
	$(CMDS) go_test go_fmt e2e_test go_verify hack_verify hack_verify_pr \
	$(DOCKER_BUILD_TARGETS) $(DOCKER_PUSH_TARGETS) $(DOCKER_RELEASE_TARGETS)

# Docker build flags
DOCKER_BUILD_FLAGS := --build-arg VCS_REF=$(GIT_COMMIT) $(DOCKER_BUILD_FLAGS)

lint:
	# @git diff-tree --check $(shell git hash-object -t tree /dev/null) HEAD $(shell ls -d * | grep -v vendor)
	@echo "Linting disabled..."

# Alias targets
###############
image:: build
build: $(CMDS) docker_build
verify: generate_verify deploy_verify hack_verify go_verify
verify_pr: hack_verify_pr
docker_build: $(DOCKER_BUILD_TARGETS)
docker_push: $(DOCKER_PUSH_TARGETS)
push_makefile: build docker_push # renamed b/c conflicts with the push in makefile.docker
multi-arch-all: docker_push
release-all: docker_release
docker_release: $(DOCKER_RELEASE_TARGETS)
artifactory_login:
	docker login $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL) --username $(ARTIFACTORY_USERNAME) --password $(ARTIFACTORY_PASSWORD)

# Code generation
#################
# This target runs all required generators against our API types.
generate: $(TYPES_FILES)
	$(HACK_DIR)/update-codegen.sh

generate_verify:
	$(HACK_DIR)/verify-codegen.sh

# Hack targets
##############
hack_verify:
	@echo Running href checker
	$(HACK_DIR)/verify-links.sh
	@echo Running errexit checker
	$(HACK_DIR)/verify-errexit.sh

hack_verify_pr:
	@echo Running helm chart version checker
	$(HACK_DIR)/verify-chart-version.sh

deploy_verify:
	@echo Running deploy-gen
	$(HACK_DIR)/verify-deploy-gen.sh

# Go targets
#################
go_verify: go_fmt go_test

$(CMDS):
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-a -tags netgo \
		-o $(DOCKERFILES)/${APP_NAME}-$@_$(GOOS)_$(GOARCH) \
		$(GOLDFLAGS) \
		./cmd/$@

go_test:
	go test -v \
	    -race \
		$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/client' | \
			grep -v '/third_party' \
		)

go_fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi


# Docker targets
################
$(DOCKER_BUILD_TARGETS):
    
	$(eval DOCKER_BUILD_CMD := $(subst docker_build_,,$@))
	$(eval WORKING_CHANGES := $(shell git status --porcelain))
	$(eval BUILD_DATE := $(shell date +%m/%d@%H:%M:%S))
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))

	$(eval DOCKER_FILE := $(DOCKERFILES)/$(DOCKER_BUILD_CMD)/Dockefile)
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_BUILD_CMD))

	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval IMAGE_NAME_S390X := ${IMAGE_REPO}/${IMAGE_NAME}-s390x:${RELEASE_TAG})
	$(eval DOCKER_FILE := $(DOCKERFILES)/$(DOCKER_BUILD_CMD)/Dockerfile$(DOCKER_FILE_EXT))

	@echo "App: $(IMAGE_NAME_ARCH) $(IMAGE_VERSION)"
	@echo "DOCKER_FILE: $(DOCKER_FILE)"
	
	# Build with a tag to the original repo.
	docker build -t $(MDELDER_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) \
           --build-arg "VCS_REF=$(VCS_REF)" \
           --build-arg "VCS_URL=$(GIT_REMOTE_URL)" \
           --build-arg "IMAGE_NAME=$(IMAGE_NAME_ARCH)" \
           --build-arg "IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION)" \
		   --build-arg "GOARCH=$(GOARCH)" \
		   -f $(DOCKER_FILE) $(DOCKERFILES)
	@echo "Built with the original repo tag."

	# Build with a tag to the new repo.
	docker build -t $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) \
           --build-arg "VCS_REF=$(VCS_REF)" \
           --build-arg "VCS_URL=$(GIT_REMOTE_URL)" \
           --build-arg "IMAGE_NAME=$(IMAGE_NAME_ARCH)" \
           --build-arg "IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION)" \
		   --build-arg "GOARCH=$(GOARCH)" \
		   -f $(DOCKER_FILE) $(DOCKERFILES)
	@echo "Built with the new repo tag."

$(DOCKER_PUSH_TARGETS):
	$(eval DOCKER_PUSH_CMD := $(subst docker_push_,,$@))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_PUSH_CMD))
	$(eval IMAGE_NAME_S390X := ${MDELDER_IMAGE_REPO}/${IMAGE_NAME}-s390x:${RELEASE_TAG})
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))

	manifest-tool inspect $(IMAGE_NAME_S390X) \
		|| (docker pull $(DEFAULT_S390X_IMAGE) \
		&& docker tag $(DEFAULT_S390X_IMAGE) $(IMAGE_NAME_S390X) \
		&& docker push $(IMAGE_NAME_S390X))

	# Push the manifest to the original mdelder repo.
	cp manifest.yaml /tmp/manifest-$(DOCKER_PUSH_CMD).yaml
	sed -i -e "s|__RELEASE_TAG__|$(RELEASE_TAG)|g" /tmp/manifest-$(DOCKER_PUSH_CMD).yaml
	sed -i -e "s|__IMAGE_NAME__|$(IMAGE_NAME)|g"  /tmp/manifest-$(DOCKER_PUSH_CMD).yaml
	sed -i -e "s|__IMAGE_REPO__|$(MDELDER_IMAGE_REPO)|g" /tmp/manifest-$(DOCKER_PUSH_CMD).yaml
	manifest-tool push from-spec /tmp/manifest-$(DOCKER_PUSH_CMD).yaml

$(DOCKER_RELEASE_TARGETS):
	$(eval DOCKER_RELEASE_CMD := $(subst docker_release_,,$@))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_RELEASE_CMD))
	$(eval IMAGE_NAME_S390X := ${MDELDER_IMAGE_REPO}/${IMAGE_NAME}-s390x:${RELEASE_TAG})
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval ARTIFACTORY_RELEASE_TAG ?= $(IMAGE_VERSION))

	# Push to original image repo.
	docker push $(MDELDER_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION)
	docker tag $(MDELDER_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) $(MDELDER_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)
	docker push $(MDELDER_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)
	@echo "Pushed image to original bluemix repo (mdelder)."

	# Push to new image repo.
	docker push $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION)
	docker tag $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(ARTIFACTORY_RELEASE_TAG)
	docker push $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(ARTIFACTORY_RELEASE_TAG)
	@echo "Pushed image to artifactory repository."


include Makefile.docker
#include Makefile.test
