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

REGISTRY := quay.io/jetstack
IMAGE_TAGS := canary

GINKGO_SKIP :=

# AppVersion is set as the AppVersion to be compiled into the controller binary.
# It's used as the default version of the 'acmesolver' image to use for ACME
# challenge requests, and any other future provider that requires additional
# image dependencies will use this same tag.
ifeq ($(APP_VERSION),)
APP_VERSION := $(if $(shell cat VERSION 2> /dev/null),$(shell cat VERSION 2> /dev/null),0.7.0)
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
# docker_retag_controller
DOCKER_RETAG_TARGETS := $(addprefix docker_retag_, $(CMDS))
## e2e test vars
KUBECTL ?= kubectl
KUBECONFIG ?= $$HOME/.kube/config

# Go build flags
GOOS := linux

GIT_COMMIT = $(shell git rev-parse --short HEAD)
GOLDFLAGS := -ldflags "-X $(PACKAGE_NAME)/pkg/util.AppGitState=${GIT_STATE} -X $(PACKAGE_NAME)/pkg/util.AppGitCommit=${GIT_COMMIT} -X $(PACKAGE_NAME)/pkg/util.AppVersion=${APP_VERSION}"

.PHONY: help verify build build-images artifactory-login push-images rhel-images \
	generate generate-verify deploy-verify \
	$(CMDS) go-test go-fmt e2e-test go-verify hack-verify hack-verify-pr \
	$(DOCKER_BUILD_TARGETS) $(DOCKER_PUSH_TARGETS) $(DOCKER_RELEASE_TARGETS) $(DOCKER_RETAG_TARGETS) \
	verify-lint verify-codegen verify-deps verify-unit \
	dep-verify verify-docs verify-chart 

# Docker build flags
DOCKER_BUILD_FLAGS := --build-arg VCS_REF=$(GIT_COMMIT) $(DOCKER_BUILD_FLAGS)

lint:
	# @git diff-tree --check $(shell git hash-object -t tree /dev/null) HEAD $(shell ls -d * | grep -v vendor)
	@echo "Linting disabled..."
	

help:
	# This Makefile provides common wrappers around Bazel invocations.
	#
	### Verify targets
	#
	# verify_lint        - run 'lint' targets
	# verify_unit        - run unit tests
	# verify_deps        - verifiy vendor/ and Gopkg.lock is up to date
	# verify_codegen     - verify generated code, including 'static deploy manifests', is up to date
	# verify_docs        - verify the generated reference docs for API types is up to date
	# verify_chart       - runs Helm chart linter (e.g. ensuring version has been bumped etc)
	#
	### Generate targets
	#
	# generate           - regenerate all generated files
	#
	### Build targets
	#
	# build				 - builds all images (controller, acmesolver, webhook)
	# e2e_test           - builds and runs end-to-end tests.
	#                      NOTE: you probably want to execute ./hack/ci/run-e2e-kind.sh instead of this target
	#

# Alias targets
###############
image:: build
build: $(CMDS) build-images

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

build-images: $(DOCKER_BUILD_TARGETS)
push-images: $(DOCKER_RELEASE_TARGETS)
rhel-images: $(DOCKER_RETAG_TARGETS)
artifactory-login:
	$(SSH_CMD) docker login $(IMAGE_REPO).$(URL) --username $(ARTIFACTORY_USERNAME) --password $(ARTIFACTORY_PASSWORD)

tunnel:
	$(shell cp rhel-buildmachines/id_rsa ~/.ssh/rhel_id_rsa)
	$(shell cp rhel-buildmachines/config ~/.ssh/config)
	$(shell chmod 0600 ~/.ssh/rhel_id_rsa)

# Code generation
#################
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
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | grep -v 'third_party/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

e2e_test:
	mkdir -p "$$(pwd)/_artifacts"
	bazel build //hack/bin:helm //test/e2e:e2e.test
	# Run e2e tests
	KUBECONFIG=$(KUBECONFIG) \
		bazel run //vendor/github.com/onsi/ginkgo/ginkgo -- \
			-nodes 20 \
			$$(bazel info bazel-genfiles)/test/e2e/e2e.test \
			-- \
			--helm-binary-path=$$(bazel info bazel-genfiles)/hack/bin/helm \
			--repo-root="$$(pwd)" \
			--report-dir="$${ARTIFACTS:-./_artifacts}" \
			--ginkgo.skip="$(GINKGO_SKIP)" \
			--kubectl-path="$(KUBECTL)"

# Generate targets
##################

generate:
	bazel run //hack:update-bazel
	bazel run //hack:update-gofmt
	bazel run //hack:update-codegen
	bazel run //hack:update-deploy-gen
	bazel run //hack:update-reference-docs
	bazel run //hack:update-deps

# Docker targets
################
$(DOCKER_BUILD_TARGETS):
	$(eval DOCKER_FILE_CMD := $(subst docker_build_,,$@))
	$(eval WORKING_CHANGES := $(shell git status --porcelain))
	$(eval BUILD_DATE := $(shell date +%m/%d@%H:%M:%S))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_FILE_CMD))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval REPO_URL := $(IMAGE_REPO).$(URL)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))

	@echo "OS = $(OS)"
	$(eval DOCKER_FILE := $(DOCKERFILES)/$(DOCKER_FILE_CMD)/Dockerfile$(DOCKER_FILE_EXT))

	@echo "App: $(IMAGE_NAME_ARCH):$(IMAGE_VERSION)"
	@echo "DOCKER_FILE: $(DOCKER_FILE)"
	
	$(eval DOCKER_BUILD_CMD := docker build -t $(REPO_URL):$(IMAGE_VERSION) \
           --build-arg "VCS_REF=$(VCS_REF)" \
           --build-arg "VCS_URL=$(GIT_REMOTE_URL)" \
           --build-arg "IMAGE_NAME=$(IMAGE_NAME_ARCH)" \
           --build-arg "IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION)" \
		   --build-arg "GOARCH=$(GOARCH)" \
		   -f $(DOCKER_FILE) $(DOCKERFILES))

ifeq ($(OS),rhel7)
	$(eval BASE_DIR := go/src/github.com/jetstack/cert-manager/)
	$(eval BASE_CMD := cd $(BASE_DIR);)
	$(SSH_CMD) mkdir -p $(BASE_DIR)$(DOCKERFILES)/$(DOCKER_FILE_CMD)
	scp $(DOCKERFILES)/$(IMAGE_NAME)_$(GOOS)_$(GOARCH) root@${TARGET}:$(BASE_DIR)$(DOCKERFILES)/$(IMAGE_NAME)_$(GOOS)_$(GOARCH)
	scp $(DOCKER_FILE) root@${TARGET}:$(BASE_DIR)$(DOCKER_FILE)

	# Building docker image.
	$(SSH_CMD) '$(BASE_CMD) $(DOCKER_BUILD_CMD)'
	@echo "Built docker image."
else
	# Building docker image.
	$(DOCKER_BUILD_CMD)
	@echo "Built docker image."
endif

$(DOCKER_RELEASE_TARGETS):
	$(eval DOCKER_RELEASE_CMD := $(subst docker_release_,,$@))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_RELEASE_CMD))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval REPO_URL := $(IMAGE_REPO).$(URL)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))

	# Pushing docker image.
	$(SSH_CMD) docker push $(REPO_URL):$(IMAGE_VERSION)
	@echo "Pushed $(REPO_URL):$(IMAGE_VERSION) to $(REPO_URL)"

ifneq ($(RETAG),)
	$(SSH_CMD) docker tag $(REPO_URL):$(IMAGE_VERSION) $(REPO_URL):$(RELEASE_TAG)
	$(SSH_CMD) docker push $(REPO_URL):$(RELEASE_TAG)
	@echo "Retagged image as $(REPO_URL):$(RELEASE_TAG) and pushed to $(REPO_URL)"
endif
	

$(DOCKER_RETAG_TARGETS):
	$(eval DOCKER_RETAG_CMD := $(subst docker_retag_,,$@))
	$(eval IMAGE_NAME := $(APP_NAME)-$(DOCKER_RETAG_CMD))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	$(eval IMAGE_NAME_ARCH := $(IMAGE_NAME)$(IMAGE_NAME_ARCH_EXT))
	$(eval REPO_URL := $(IMAGE_REPO).$(URL)/$(NAMESPACE)/$(IMAGE_NAME_ARCH))
	$(eval IMAGE_VERSION_RHEL ?= $(APP_VERSION)-$(GIT_COMMIT)$(OPENSHIFT_TAG))

	docker tag $(REPO_URL):$(IMAGE_VERSION) $(REPO_URL):$(IMAGE_VERSION_RHEL)
	docker push $(REPO_URL):$(IMAGE_VERSION_RHEL)
	@echo "Retagged image as $(REPO_URL):$(IMAGE_VERSION_RHEL) and pushed to $(REPO_URL)"

include Makefile.docker
#include Makefile.test
