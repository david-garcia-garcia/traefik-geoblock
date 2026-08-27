---
url: https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/README.md
title: Yaegi README
fetched: 2026-08-26
authority: official
ref: github.com/traefik/yaegi@fcb76d1ece0c3edc2548c39aa5b170475d2261bb:README.md
---

Yaegi is a Go interpreter. Claimed feature: complete support of the Go specification.

`Eval` can run `import "fmt"` and can interpret Go packages, directories, or files.

Limitations: “Go modules are not supported yet. Until that, it is necessary to install the source into `$GOPATH/src/github.com/traefik/yaegi` to pass all the tests.”

This is GOPATH-style package loading. It does not say a module tree may contain only one package.
