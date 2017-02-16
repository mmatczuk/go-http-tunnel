OUTPUT_DIR = build
OS = "darwin freebsd linux windows"
ARCH = "amd64 arm"
OSARCH = "!darwin/arm !windows/arm"

GIT_COMMIT = $(shell git describe --always)

all: check

.PHONY: check
check:
	gofmt -s -l . | ifne false
	go vet ./...
	golint ./...
	go build ./...
	go test -race ./...

.PHONY: clean
clean:
	go clean ./...
	rm -rf ${OUTPUT_DIR}

.PHONY: devtools
devtools:
	go get -u github.com/golang/lint/golint
	go get -u github.com/golang/mock/gomock
	go get -u github.com/mitchellh/gox
	go get -u github.com/tcnksm/ghr

.PHONY: release
release: clean check build package publish

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