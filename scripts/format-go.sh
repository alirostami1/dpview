#!/bin/sh
set -eu

git ls-files -z -- "*.go" | xargs -0r gofmt -w
