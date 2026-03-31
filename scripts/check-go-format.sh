#!/bin/sh
set -eu

unformatted="$(git ls-files -z -- "*.go" | xargs -0r gofmt -l)"
if [ -n "$unformatted" ]; then
  echo "The following Go files are not gofmt-formatted:"
  echo "$unformatted"
  exit 1
fi
