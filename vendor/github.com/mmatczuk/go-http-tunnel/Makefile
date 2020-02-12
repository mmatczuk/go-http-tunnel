
GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

LINT_IGNORE := "/id/\|/tunnelmock/\|/vendor/"

all: clean check test

.PHONY: clean
clean:
	@go clean -r

.PHONY: fmt
fmt:
	@go fmt ./...

.PHONY: check
check: .check-fmt .check-vet .check-lint .check-ineffassign .check-static .check-misspell .check-vendor

.PHONY: .check-fmt
.check-fmt:
	$(eval FMT_LOG := $(shell mktemp -t gofmt.XXXXX))
	@cat /dev/null > $(FMT_LOG)
	@gofmt -e -s -l -d $(GO_FILES) > $(FMT_LOG) || true
	@[ ! -s "$(FMT_LOG)" ] || (echo "$@ failed:" | cat - $(FMT_LOG) && false)

.PHONY: .check-vet
.check-vet:
	@go vet ./...

.PHONY: .check-lint
.check-lint:
	$(eval LINT_LOG := $(shell mktemp -t golint.XXXXX))
	@cat /dev/null > $(LINT_LOG)
	@$(foreach pkg, $(GO_FILES), golint $(pkg | grep -v $LINT_IGNORE) >> $(LINT_LOG) || true;)
	@[ ! -s "$(LINT_LOG)" ] || (echo "$@ failed:" | cat - $(LINT_LOG) && false)


.PHONY: .check-ineffassign
.check-ineffassign:
	@ineffassign ./

.PHONY: .check-misspell
.check-misspell:
	@misspell ./...

.PHONY: .check-mega
.check-static:
	@staticcheck -checks ['SA1006','ST1005'] ./...

.PHONY: .check-vendor
.check-vendor:
	@dep ensure -no-vendor -dry-run

.PHONY: test
test:
	@echo "==> Running tests (race)..."
	@go test -cover -race ./...

.PHONY: get-deps
get-deps:
	@echo "==> Installing dependencies..."
	@dep ensure

.PHONY: get-tools
get-tools:
	@echo "==> Installing tools..."
	@go get -u github.com/golang/dep/cmd/dep
	@go get -u golang.org/x/lint/golint
	@go get -u github.com/golang/mock/gomock

	@go get -u github.com/client9/misspell/cmd/misspell
	@go get -u github.com/gordonklaus/ineffassign
	@go get -u github.com/mitchellh/gox
	@go get -u github.com/tcnksm/ghr
	@go get -u honnef.co/go/tools/cmd/staticcheck

OUTPUT_DIR = build
OS = "darwin freebsd linux windows"
ARCH = "386 amd64 arm"
OSARCH = "!darwin/386 !darwin/arm !windows/arm"
GIT_COMMIT = $(shell git describe --always)

.PHONY: release
release: check test clean build package

.PHONY: build
build:
	mkdir ${OUTPUT_DIR}
	CGO_ENABLED=0 GOARM=5 gox -ldflags "-w -X main.version=$(GIT_COMMIT)" \
	-os=${OS} -arch=${ARCH} -osarch=${OSARCH} -output "${OUTPUT_DIR}/pkg/{{.OS}}_{{.Arch}}/{{.Dir}}" \
	./cmd/tunnel ./cmd/tunneld

.PHONY: package
package:
	mkdir ${OUTPUT_DIR}/dist
	cd ${OUTPUT_DIR}/pkg/; for osarch in *; do (cd $$osarch; tar zcvf ../../dist/tunnel_$$osarch.tar.gz ./*); done;
	cd ${OUTPUT_DIR}/dist; sha256sum * > ./SHA256SUMS

.PHONY: publish
publish:
	ghr -recreate -u mmatczuk -t ${GITHUB_TOKEN} -r go-http-tunnel pre-release ${OUTPUT_DIR}/dist
