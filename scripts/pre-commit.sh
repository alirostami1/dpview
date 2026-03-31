#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

mapfile -d '' staged_files < <(git diff --cached --name-only -z --diff-filter=ACMR)

if ((${#staged_files[@]} == 0)); then
  exit 0
fi

go_files=()
ts_files=()
prettier_files=()
lua_files=()

for file in "${staged_files[@]}"; do
  case "$file" in
    *.go)
      go_files+=("$file")
      ;;
    *.ts)
      ts_files+=("$file")
      prettier_files+=("$file")
      ;;
    *.js|*.mjs|*.cjs|*.css|*.html|*.json|*.md|*.yml|*.yaml)
      prettier_files+=("$file")
      ;;
    *.lua)
      lua_files+=("$file")
      ;;
  esac
done

if ((${#go_files[@]} > 0)); then
  unformatted="$(gofmt -l "${go_files[@]}")"
  if [[ -n "$unformatted" ]]; then
    echo "The following staged Go files are not gofmt-formatted:"
    echo "$unformatted"
    echo "Run: npm run format:go"
    exit 1
  fi

  echo "Running go vet..."
  go vet ./...
fi

if ((${#prettier_files[@]} > 0)); then
  echo "Checking Prettier formatting on staged files..."
  npx --no-install prettier --check "${prettier_files[@]}"
fi

if ((${#lua_files[@]} > 0)); then
  echo "Checking Stylua formatting on staged files..."
  stylua --check "${lua_files[@]}"
fi

if ((${#ts_files[@]} > 0)); then
  echo "Running ESLint on staged TypeScript files..."
  npx --no-install eslint "${ts_files[@]}"

  echo "Running TypeScript typecheck..."
  npm run typecheck
fi
