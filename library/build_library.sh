#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "$(go env GOOS)" in
windows) ext="dll" ;;
darwin) ext="dylib" ;;
*) ext="so" ;;
esac
CGO_ENABLED=1 go build -buildmode=c-shared -o "../output/pg_extension.${ext}" .
