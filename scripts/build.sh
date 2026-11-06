#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
if [ "$(bun --version)" != "$(cat .bun-version)" ]; then
  echo "Use the Bun version pinned in .bun-version" >&2
  exit 1
fi
bun install --cwd web --frozen-lockfile
bun run --cwd web build
go build -trimpath -o bin/velnix ./cmd/velnix
