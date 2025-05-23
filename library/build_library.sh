#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "$(go env GOOS)" in
windows) ext="dll" ;;
darwin) ext="dylib" ;;
*) ext="so" ;;
esac

mkdir -p temp_lib
trap 'rm -rf temp_lib' EXIT

cp ./*.* ./temp_lib

for f in temp_lib/*.go; do
  sed 's/^package library_package_name$/package main/' "$f" > "$f".tmp
  mv "$f".tmp "$f"
done
printf "module github.com/dolthub/pg_extension\n\ngo 1.24" > ./temp_lib/go.mod

(
    cd temp_lib
    CGO_ENABLED=1 go build -buildmode=c-shared -o "../../output/pg_extension.${ext}" .
)
