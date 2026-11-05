generate:
	go generate
.PHONY: generate

test:
	go test -v -cover .
.PHONY: test

# Packages whose tests pass under Yaegi v0.16.1, the interpreter Traefik runs the plugin in.
# yaegi test needs a GOPATH layout: $GOPATH/src/github.com/david-garcia-garcia/traefik-geoblock.
YAEGI_MODULE := github.com/david-garcia-garcia/traefik-geoblock
YAEGI_PACKAGES := pkg/fileutils pkg/dbutils pkg/logging

test-yaegi:
	for p in $(YAEGI_PACKAGES); do GO111MODULE=off yaegi test -v $(YAEGI_MODULE)/$$p || exit 1; done
.PHONY: test-yaegi