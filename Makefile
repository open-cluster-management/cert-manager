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

## e2e test vars
KUBECTL ?= kubectl
KUBECONFIG ?= $$HOME/.kube/config

# Get a list of all binaries to be built
CMDS := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)

.PHONY: help build verify push $(CMDS) e2e_test images images_push \
	verify_lint verify_unit verify_deps verify_codegen verify_docs verify_chart \

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
	# controller         - build a binary of the 'controller'
	# injectorcontroller - build a binary of the 'injectorcontroller'
	# webhook            - build a binary of the 'webhook'
	# acmesolver         - build a binary of the 'acmesolver'
	# e2e_test           - builds and runs end-to-end tests.
	#                      NOTE: you probably want to execute ./hack/ci/run-e2e-kind.sh instead of this target
	# images             - builds docker images for all of the components, saving them in your Docker daemon
	# images_push        - pushes docker images to the target registry
	#
	# Image targets can be run with optional args DOCKER_REPO and DOCKER_TAG:
	#
	#     make images DOCKER_REPO=quay.io/yourusername DOCKER_TAG=experimental-tag
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
artifactory-login:
	$(SSH_CMD) docker login $(IMAGE_REPO).$(URL) --username $(ARTIFACTORY_USERNAME) --password $(ARTIFACTORY_PASSWORD)

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

BAZEL_IMAGE_ENV := APP_VERSION=$(APP_VERSION) DOCKER_REPO=$(DOCKER_REPO) DOCKER_TAG=$(APP_VERSION)
images:
	$(BAZEL_IMAGE_ENV) \
		bazel run //:images

images_push: images
	# we do not use the :push target as Quay.io does not support v2.2
	# manifests for Docker images, and rules_docker only supports 2.2+
	# https://github.com/moby/buildkit/issues/409#issuecomment-394757219
	# source the bazel workspace environment
	eval $$($(BAZEL_IMAGE_ENV) ./hack/print-workspace-status.sh | tr ' ' '='); \
	docker tag "$${STABLE_DOCKER_REPO}/cert-manager-acmesolver-amd64:$${STABLE_DOCKER_TAG}" "$${STABLE_DOCKER_REPO}/cert-manager-acmesolver:$${STABLE_DOCKER_TAG}"; \
	docker tag "$${STABLE_DOCKER_REPO}/cert-manager-controller-amd64:$${STABLE_DOCKER_TAG}" "$${STABLE_DOCKER_REPO}/cert-manager-controller:$${STABLE_DOCKER_TAG}"; \
	docker tag "$${STABLE_DOCKER_REPO}/cert-manager-injectorcontroller-amd64:$${STABLE_DOCKER_TAG}" "$${STABLE_DOCKER_REPO}/cert-manager-injectorcontroller:$${STABLE_DOCKER_TAG}"; \
	docker tag "$${STABLE_DOCKER_REPO}/cert-manager-webhook-amd64:$${STABLE_DOCKER_TAG}" "$${STABLE_DOCKER_REPO}/cert-manager-webhook:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-acmesolver:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-controller:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-injectorcontroller:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-webhook:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-acmesolver-arm64:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-controller-arm64:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-injectorcontroller-arm64:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-webhook-arm64:$${STABLE_DOCKER_TAG}";
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-acmesolver-arm:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-controller-arm:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-injectorcontroller-arm:$${STABLE_DOCKER_TAG}"; \
	docker push "$${STABLE_DOCKER_REPO}/cert-manager-webhook-arm:$${STABLE_DOCKER_TAG}";
