#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "$(go env GOOS)" in
windows) ext="dll" ;;
darwin) ext="dylib" ;;
*) ext="so" ;;
esac

# For now, other platforms directly embed the library, but removing this check will allow them to build a shared library as well
if [[ "$(go env GOOS)" != "windows" ]]; then
    exit 0
fi

# For Windows, we also need to build the definition file
if [[ "$(go env GOOS)" == "windows" ]]; then
  VSWHERE='"%ProgramFiles(x86)%/Microsoft Visual Studio/Installer/vswhere.exe"'
  VSROOT=$(cmd.exe /c "\"$VSWHERE\" -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath" | tr -d '\r')
  MSVCVER=$(ls "$VSROOT/VC/Tools/MSVC" | sort -V | tail -n 1)
  LINK="$VSROOT/VC/Tools/MSVC/$MSVCVER/bin/Hostx64/x64/link.exe"
  OUTDIR=../output
  mkdir -p "$OUTDIR"
  OUT_WIN=$(wslpath -w "$OUTDIR" 2>/dev/null || cygpath -w "$OUTDIR")
  cmd.exe /c "call \"${VSROOT}\VC\Auxiliary\Build\vcvars64.bat\" && \"${LINK}\" /DLL /NOENTRY /DEF:postgres.def /OUT:${OUT_WIN}\\postgres.exe"
fi

# MacOS requires that exported functions are present in the calling binary, but other platforms use a dynamic library.
# To account for this, we put the exported functions in a normal package by default.
# For MacOS, this works just fine as we import the package.
# To make a dynamic library, we copy the files into a temporary directory and modify the files to create a valid library.
# This lets us use the same code for both scenarios.
mkdir -p temp_lib
trap 'rm -rf temp_lib' EXIT

cp ./*.* ./temp_lib

for f in temp_lib/*.go; do
  sed 's/^package extension_cgo$/package main/' "$f" > "$f".tmp
  mv "$f".tmp "$f"
done
printf "module github.com/dolthub/pg_extension\n\ngo 1.24" > ./temp_lib/go.mod

(
    cd temp_lib
    CGO_ENABLED=1 go build -buildmode=c-shared -o "../../output/pg_extension.${ext}" .
)
