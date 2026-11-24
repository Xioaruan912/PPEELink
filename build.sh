#!/usr/bin/env bash
set -euo pipefail
mkdir -p dist

# Windows built with -H=windowsgui so background daemon never opens a console window.
echo "Building windows/amd64 -> dist/ppeelink-user-agent-windows-amd64.exe"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-w -s -H=windowsgui" -o "dist/ppeelink-user-agent-windows-amd64.exe" .

targets=(
  "darwin arm64 ppeelink-user-agent-darwin-arm64"
  "darwin amd64 ppeelink-user-agent-darwin-amd64"
  "linux amd64 ppeelink-user-agent-linux-amd64"
  "linux arm64 ppeelink-user-agent-linux-arm64"
)

for item in "${targets[@]}"; do
  read -r os arch out <<<"$item"
  echo "Building $os/$arch -> dist/$out"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -ldflags="-w -s" -o "dist/$out" .
done

ls -lh dist/
