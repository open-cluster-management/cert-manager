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
# Copyright (c) 2020 Red Hat, Inc.

# CICD BUILD HARNESS
####################
-include $(shell curl -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
####################

.PHONY: default
default::
	@echo "Build Harness Bootstrapped"

# Look at what these mean
GOLDFLAGS := -ldflags "-X $(PACKAGE_NAME)/pkg/util.AppGitState=${GIT_STATE} -X $(PACKAGE_NAME)/pkg/util.AppGitCommit=${GIT_COMMIT} -X $(PACKAGE_NAME)/pkg/util.AppVersion=${APP_VERSION}"

.PHONY: lint build go-binary docker-image docker-push rhel-image \
	go-test go-fmt go-verify go-coverage

# Docker build flags
DOCKER_BUILD_FLAGS := --build-arg VCS_REF=$(GIT_COMMIT) $(DOCKER_BUILD_FLAGS)

lint:
	@git diff-tree --check $(shell git hash-object -t tree /dev/null) HEAD $(shell ls -d * | grep -v vendor | grep -v docs | grep -v deploy | grep -v hack)

copyright-check:
	./build/copyright-check.sh $(TRAVIS_BRANCH) $(TRAVIS_PULL_REQUEST_BRANCH)

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
	$(shell go test -v \
		$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/client' | \
			grep -v '/third_party' | \
			grep -v '/docs/' \
		) > results.txt)
	$(eval FAILURES=$(shell cat results.txt | grep "FAIL:"))
	cat results.txt
	@$(if $(strip $(FAILURES)), echo "One or more unit tests failed. Failures: $(FAILURES)"; exit 1, echo "All unit tests passed successfully."; exit 0)

go-coverage:
	$(shell go test -coverprofile=coverage.out -json ./...\
                $$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/client' | \
			grep -v '/third_party' | \
			grep -v '/docs/' \
		) > report.json)
	gosec -fmt sonarqube -out gosec.json -no-fail ./...
	sonar-scanner --debug || echo "Sonar scanner is not available"

go-fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | grep -v 'third_party/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

