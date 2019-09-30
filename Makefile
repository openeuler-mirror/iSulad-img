ISULAD_KIT_BIN=./isulad_kit
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
GOTMPDIR=/tmp/isulad-kit

# ifeq ($(DISABLE_CGO), 1)
		override BUILDTAGS = containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp
# endif

.PHONY: all isulad_kit static  clean

all: isulad_kit

isulad_kit: link
	echo $(GOPATH)
	echo $(CURDIR)
	mkdir -p ${GOTMPDIR}
	$(GPGME_ENV) go build ${GO_DYN_FLAGS} -ldflags "-tmpdir ${GOTMPDIR} -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o isulad_kit ./cmd/isulad_kit
	rm -rf ${GOTMPDIR}

static: link
	echo $(GOPATH)
	echo $(CURDIR)
	mkdir -p ${GOTMPDIR}
	$(GPGME_ENV) go build -ldflags "-tmpdir ${GOTMPDIR} -extldflags \"-static\" -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o isulad_kit ./cmd/isulad_kit
	rm -rf ${GOTMPDIR}

clean:
	rm -rf ${ISULAD_KIT_BIN}

install:
	install -d -m 755 ${INSTALLDIR}
	install -m 755 ${ISULAD_KIT_BIN} ${INSTALLDIR}/isulad_kit
	install -d -m 755 ${CONTAINERSSYSCONFIGDIR}
	install -m 644 default-policy.json ${CONTAINERSSYSCONFIGDIR}/policy.json

link:
	ln -sfn $(CURDIR)/vendor $(CURDIR)/src
