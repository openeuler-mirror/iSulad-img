ISULAD_KIT_BIN=./isulad-img
BUILT=$(shell echo `date +'%Y%m%d-%H:%M:%S'`)

ifeq ($(shell uname),Darwin)
PREFIX ?= ${DESTDIR}/usr/local
DARWIN_BUILD_TAG=containers_image_ostree_stub
# On macOS, (brew install gpgme) installs it within /usr/local, but /usr/local/include is not in the default search path.
# Rather than hard-code this directory, use gpgme-config. Sadly that must be done at the top-level user
# instead of locally in the gpgme subpackage, because cgo supports only pkg-config, not general shell scripts,
# and gpgme does not install a pkg-config file.
# If gpgme is not installed or gpgme-config can¡¯t be found for other reasons, the error is silently ignored
# (and the user will probably find out because the cgo compilation will fail).
GPGME_ENV := CGO_CFLAGS="$(shell gpgme-config --cflags 2>/dev/null)" CGO_LDFLAGS="$(shell gpgme-config --libs 2>/dev/null)"
else
PREFIX ?= ${DESTDIR}/usr
endif

INSTALLDIR=${PREFIX}/bin
CONTAINERSSYSCONFIGDIR=${DESTDIR}/etc/containers

ifeq ($(shell go env GOOS), linux)
  GO_DYN_FLAGS="-buildmode=pie"
endif

export GOPATH := $(CURDIR)

BTRFS_BUILD_TAG = $(shell hack/btrfs_tag.sh)
LIBDM_BUILD_TAG = $(shell hack/libdm_tag.sh)
LOCAL_BUILD_TAGS = $(BTRFS_BUILD_TAG) $(LIBDM_BUILD_TAG) $(DARWIN_BUILD_TAG)
BUILDTAGS += $(LOCAL_BUILD_TAGS)
GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)
GO_LDFLAGS="-s -w -extldflags -static -X main.gitCommit=${GIT_COMMIT} -X main.built=${BUILT}"
GOTMPDIR=/tmp/isulad-img

# ifeq ($(DISABLE_CGO), 1)
		override BUILDTAGS = containers_image_ostree_stub exclude_graphdriver_btrfs containers_image_openpgp
# endif

.PHONY: all isulad_img static  clean

all: isulad_img

isulad_img: link
	echo $(GOPATH)
	echo $(CURDIR)
	rm -rf $(CURDIR)/src/isula-image/isula
	mkdir -p $(CURDIR)/src/isula-image/
	cp -rf isula $(CURDIR)/src/isula-image/
	mkdir -p ${GOTMPDIR}
	$(GPGME_ENV) go build ${GO_DYN_FLAGS} -ldflags "-extldflags -zrelro -extldflags -znow -extldflags -ftrapv -tmpdir ${GOTMPDIR} -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o isulad-img ./cmd/isulad_img
	rm -rf ${GOTMPDIR}
	rm -rf $(CURDIR)/src/isula-image/isula

static: link
	echo $(GOPATH)
	echo $(CURDIR)
	rm -rf $(CURDIR)/src/isula-image/isula
	mkdir -p $(CURDIR)/src/isula-image/
	cp -rf isula $(CURDIR)/src/isula-image/
	mkdir -p ${GOTMPDIR}
	$(GPGME_ENV) go build -ldflags "-tmpdir ${GOTMPDIR} -extldflags \"-static\" -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o isulad-img ./cmd/isulad_img
	rm -rf ${GOTMPDIR}
	rm -rf $(CURDIR)/src/isula-image/isula

unit-test: link
	echo $(GOPATH)
	echo $(CURDIR)
	rm -rf $(CURDIR)/src/isula-image
	mkdir -p $(CURDIR)/src/isula-image/cmd
	cp -rf $(CURDIR)/cmd/isulad_img $(CURDIR)/src/isula-image/cmd/
	cp -rf $(CURDIR)/isula $(CURDIR)/src/isula-image/
	mkdir -p ${GOTMPDIR}
	$(GPGME_ENV) go test -count=1 ${GO_DYN_FLAGS} -ldflags "-extldflags -zrelro -extldflags -znow -tmpdir ${GOTMPDIR} -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -v -coverpkg=github.com/containers/storage/drivers/devmapper github.com/containers/storage/drivers/devmapper
	$(GPGME_ENV) go test -count=1 ${GO_DYN_FLAGS} -ldflags "-extldflags -zrelro -extldflags -znow -tmpdir ${GOTMPDIR} -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -v -coverpkg=isula-image/cmd/isulad_img isula-image/cmd/isulad_img
	rm -rf ${GOTMPDIR}
	rm -rf $(CURDIR)/src/isula-image
	rm -f $(CURDIR)/src

proto:
	protoc --go_out=plugins=grpc:. ./isula/isula_image.proto

clean:
	rm -rf ${ISULAD_KIT_BIN}
	rm -rf $(CURDIR)/src/isula-image
	rm -f $(CURDIR)/src

install:
	install -d -m 755 ${INSTALLDIR}
	install -m 755 ${ISULAD_KIT_BIN} ${INSTALLDIR}/isulad-img
	install -d -m 755 ${CONTAINERSSYSCONFIGDIR}
	install -m 644 default-policy.json ${CONTAINERSSYSCONFIGDIR}/policy.json

link:
	ln -sfn $(CURDIR)/vendor $(CURDIR)/src
