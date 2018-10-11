include Configfile
# Copyright 2018 The Jetstack cert-manager contributors.
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


REGISTRY := quay.io/jetstack
IMAGE_TAGS := canary

# Domain name to use in e2e tests. This is important for ACME HTTP01 e2e tests,
# which require a domain that resolves to the ingress controller to be used for
# e2e tests.
E2E_NGINX_CERTIFICATE_DOMAIN=
KUBECONFIG ?= $$HOME/.kube/config
PEBBLE_IMAGE_REPO=quay.io/munnerz/pebble

# AppVersion is set as the AppVersion to be compiled into the controller binary.
# It's used as the default version of the 'acmesolver' image to use for ACME
# challenge requests, and any other future provider that requires additional
# image dependencies will use this same tag.
ifeq ($(APP_VERSION),)
APP_VERSION := $(if $(shell cat VERSION 2> /dev/null),$(shell cat VERSION 2> /dev/null),0.5.0)
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

.PHONY: verify build docker-build docker-push-manifest build-push-manifest \
	generate generate-verify deploy-verify artifactory-login docker-push-images release-all \
	$(CMDS) go-test go-fmt e2e-test go-verify hack-verify hack-verify-pr \
	$(DOCKER_BUILD_TARGETS) $(DOCKER_PUSH_TARGETS) $(DOCKER_RELEASE_TARGETS) \
	verify-lint verify-codegen verify-deps verify-unit \
	dep-verify verify-docs verify-chart 

# Docker build flags
DOCKER_BUILD_FLAGS := --build-arg VCS_REF=$(GIT_COMMIT) $(DOCKER_BUILD_FLAGS)

lint:
	# @git diff-tree --check $(shell git hash-object -t tree /dev/null) HEAD $(shell ls -d * | grep -v vendor)
	@echo "Linting disabled..."

# Alias targets
###############
image:: build
build: $(CMDS) docker-build

verify: verify-lint verify-codegen verify-deps verify-unit

verify-lint: hack-verify go-fmt
verify-unit: go-test
verify-deps: dep-verify
verify-codegen: generate-verify deploy-verify 
verify-pr: hack-verify-pr

# requires docker
verify-docs:
	$(HACK_DIR)/verify-reference-docs.sh
# requires docker
verify-chart:
	$(HACK_DIR)/verify-chart-version.sh

docker-build: $(DOCKER_BUILD_TARGETS)
docker-push-manifest: $(DOCKER_PUSH_TARGETS)
build-push-manifest: build docker-push-manifest # renamed b/c conflicts with the push in makefile.docker
multi-arch-all: docker-push-manifest
release-all: docker-push-images
docker-push-images: $(DOCKER_RELEASE_TARGETS)
artifactory-login:
	$(SSH_CMD) docker login $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL) --username $(ARTIFACTORY_USERNAME) --password $(ARTIFACTORY_PASSWORD)

tunnel:
	$(shell cp rhel-buildmachines/id_rsa ~/.ssh/rhel_id_rsa)
	$(shell cp rhel-buildmachines/config ~/.ssh/config)
	$(shell chmod 0600 ~/.ssh/rhel_id_rsa)

# Code generation
#################
# This target runs all required generators against our API types.
generate: $(TYPES_FILES)
	$(HACK_DIR)/update-codegen.sh

generate-verify:
	$(HACK_DIR)/verify-codegen.sh

# Hack targets
##############
hack-verify:
	@echo Running boilerplate header checker
	$(HACK_DIR)/verify_boilerplate.py
	@echo Running href checker
	$(HACK_DIR)/verify-links.sh
	@echo Running errexit checker
	$(HACK_DIR)/verify-errexit.sh

hack-verify-pr:
	@echo Running helm chart version checker
	$(HACK_DIR)/verify-chart-version.sh
	@echo Running reference docs checker
	IMAGE=eu.gcr.io/jetstack-build-infra/gen-apidocs-img $(HACK_DIR)/verify-reference-docs.sh

deploy-verify:
	@echo Running deploy-gen
	$(HACK_DIR)/verify-deploy-gen.sh

# Go targets
#################
go-verify: go-fmt go-test

dep-verify:
	@echo Running dep
	$(HACK_DIR)/verify-deps.sh

$(CMDS):
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-a -tags netgo \
		-o $(DOCKERFILES)/${APP_NAME}-$@_$(GOOS)_$(GOARCH) \
		$(GOLDFLAGS) \
		./cmd/$@

go-test:
	go test -v \
	    -race \
		$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/client' | \
			grep -v '/third_party' | \
			grep -v '/docs/generated' \
		)

go-fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

e2e_test:
	# Build the e2e tests
	go test -o e2e-tests -c ./test/e2e
	mkdir -p "$$(pwd)/_artifacts"
	# TODO: make these paths configurable
	# Run e2e tests
	KUBECONFIG=$(KUBECONFIG) CERTMANAGERCONFIG=$(KUBECONFIG) \
		./e2e-tests \
			-acme-nginx-certificate-domain=$(E2E_NGINX_CERTIFICATE_DOMAIN) \
			-cloudflare-email=$${CLOUDFLARE_E2E_EMAIL} \
			-cloudflare-api-key=$${CLOUDFLARE_E2E_API_TOKEN} \
			-acme-cloudflare-domain=$${CLOUDFLARE_E2E_DOMAIN} \
			-pebble-image-repo=$(PEBBLE_IMAGE_REPO) \
			-report-dir="$${ARTIFACTS:-./_artifacts}"

# Docker targets
################
$(DOCKER_BUILD_TARGETS):
	$(eval DOCKER_FILE_CMD := $(subst docker_build_,,$@))
	$(eval WORKING_CHANGES := $(shell git status --porcelain))
	$(eval BUILD_DATE := $(shell date +%m/%d@%H:%M:%S))
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))

	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT)$(OPENSHIFT_TAG))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_FILE_CMD))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval IMAGE_NAME_S390X := ${MDELDER_IMAGE_REPO}/${IMAGE_NAME}-s390x:${RELEASE_TAG})
	
	@echo "OS = $(OS)"
	$(eval DOCKER_FILE := $(DOCKERFILES)/$(DOCKER_FILE_CMD)/Dockerfile$(DOCKER_FILE_EXT))

	@echo "App: $(IMAGE_NAME_ARCH):$(IMAGE_VERSION)"
	@echo "DOCKER_FILE: $(DOCKER_FILE)"
	
	DOCKER_BUILD_CMD := docker build -t $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) \
           --build-arg "VCS_REF=$(VCS_REF)" \
           --build-arg "VCS_URL=$(GIT_REMOTE_URL)" \
           --build-arg "IMAGE_NAME=$(IMAGE_NAME_ARCH)" \
           --build-arg "IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION)" \
		   --build-arg "GOARCH=$(GOARCH)" \
		   -f $(DOCKER_FILE) $(DOCKERFILES)

ifeq ($(OS),rhel7)
	$(eval BASE_DIR := go/src/github.com/jetstack/cert-manager/)
	$(eval BASE_CMD := cd $(BASE_DIR);)
	$(SSH_CMD) mkdir -p $(BASE_DIR)$(DOCKERFILES)/$(DOCKER_FILE_CMD)
	scp $(DOCKERFILES)/$(IMAGE_NAME)_$(GOOS)_$(GOARCH) cloudusr@${TARGET}:$(BASE_DIR)$(DOCKERFILES)/$(IMAGE_NAME)_$(GOOS)_$(GOARCH)
	scp $(DOCKER_FILE) cloudusr@${TARGET}:$(BASE_DIR)$(DOCKER_FILE)

	# Build with a tag to the new repo.
	$(SSH_CMD) '$(BASE_CMD) $(DOCKER_BUILD_CMD)'
	@echo "Built with the new repo tag."
else
	# Build with a tag to the new repo.
	$(DOCKER_BUILD_CMD)
	@echo "Built with the new repo tag."
endif

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
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT)$(OPENSHIFT_TAG))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval ARTIFACTORY_RELEASE_TAG ?= $(IMAGE_VERSION))

	# Push to new image repo.
	$(SSH_CMD) docker push $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION)
	$(SSH_CMD) docker tag $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(ARTIFACTORY_RELEASE_TAG)
	$(SSH_CMD) docker push $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)/$(IMAGE_NAME_ARCH):$(ARTIFACTORY_RELEASE_TAG)
	@echo "Pushed image to image repo: $(ARTIFACTORY_IMAGE_REPO).$(ARTIFACTORY_URL)/$(ARTIFACTORY_NAMESPACE)"


include Makefile.docker
#include Makefile.test
