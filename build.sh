#!/bin/sh
set -eu
cd "$(dirname "$0")"
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/agent-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/agent-linux-arm64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags='-s -w' -o dist/agent-linux-armv7 .
AGENT_RELEASE_VERSION=$(go run . --version)
export AGENT_RELEASE_VERSION
python3 - <<'PY'
from pathlib import Path
import hashlib,json,os
root=Path('dist')
files={'linux/amd64':'agent-linux-amd64','linux/arm64':'agent-linux-arm64','linux/arm':'agent-linux-armv7'}
manifest={'version':os.environ['AGENT_RELEASE_VERSION'],'artifacts':{key:{'file':name,'sha256':hashlib.sha256((root/name).read_bytes()).hexdigest()} for key,name in files.items()}}
(root/'release.json').write_text(json.dumps(manifest,indent=2)+'\n')
PY
