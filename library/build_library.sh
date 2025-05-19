#!/usr/bin/env bash
set -euo pipefail

case "$(go env GOOS)" in
windows) ext="dll" ;;
darwin) ext="dylib" ;;
*) ext="so" ;;
esac
go build -buildmode=c-shared -o "../output/pg_extension.${ext}" "$@"
