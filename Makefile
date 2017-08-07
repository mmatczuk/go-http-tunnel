OUTPUT_DIR = build
OS = "darwin freebsd linux windows"
ARCH = "amd64 arm"
OSARCH = "!darwin/arm !windows/arm"
GIT_COMMIT = $(shell git describe --always)

all: check test

.PHONY: check
check:
	gofmt -s -l . | ifne false
	go vet ./...
	golint ./...
	misspell ./...
	ineffassign .

.PHONY: test
test:
	go test -cover -race ./...

.PHONY: release
release: check test clean build package

.PHONY: clean
clean:
	go clean ./...
	rm -rf ${OUTPUT_DIR}

.PHONY: build
build:
	mkdir ${OUTPUT_DIR}
	GOARM=5 gox -ldflags "-X main.version=$(GIT_COMMIT)" \
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

.PHONY: get-deps
get-deps:
	go get -t ./...

	go get -u github.com/golang/lint/golint
	go get -u github.com/golang/mock/gomock
	go get -u github.com/client9/misspell/cmd/misspell
	go get -u github.com/gordonklaus/ineffassign

	go get -u github.com/mitchellh/gox
	go get -u github.com/tcnksm/ghr

