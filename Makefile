GO	= go
EXT	=

PLAKAR	= plakar
VERSION	= v1.0.0

all: build

build:
#	${GO} build -v -o b2-importer${EXT} ./plugin/importer
#	${GO} build -v -o b2-exporter${EXT} ./plugin/exporter
	${GO} build -v -o b2-storage${EXT} ./plugin/storage

package: build
	rm -f b2_${VERSION}_*.ptar
	${PLAKAR} pkg create ./manifest.yaml ${VERSION}

uninstall:
	-${PLAKAR} pkg rm b2

install: package
	${PLAKAR} pkg add ./b2_${VERSION}_*.ptar

reinstall: uninstall install

test:
	${GO} test -v ./...

check: test

clean:
	rm -f b2-importer${EXT} b2-exporter${EXT} b2-storage${EXT} b2_${VERSION}*.ptar

.PHONY: all build package uninstall install reinstall test check clean
