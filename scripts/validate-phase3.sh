#!/usr/bin/env bash
# Explicit local acceptance, with existing prerequisites only. No downloads,
# packages, service changes, hosts writes or trust-store imports occur here.
set -euo pipefail
cd -- "$(dirname -- "$0")/.."
if [[ $# != 1 || ! -x "$1" ]]; then
    echo 'Usage: scripts/validate-phase3.sh /absolute/existing/mkcert-v1.4.4' >&2
    exit 2
fi
if [[ $(id -u) == 0 ]]; then
    echo 'Run as an ordinary user; Phase 3 workloads are rootless.' >&2
    exit 2
fi
for tool in go nginx; do
    command -v "$tool" >/dev/null || { echo "Missing prerequisite: $tool" >&2; exit 2; }
done
for runtime in /usr/bin/php8.3 /usr/bin/php8.5 /usr/sbin/php-fpm8.3 /usr/sbin/php-fpm8.5; do
    [[ -x "$runtime" ]] || { echo "Missing two-version acceptance prerequisite: $runtime" >&2; exit 2; }
done
export VELNIX_TEST_MKCERT
VELNIX_TEST_MKCERT=$(realpath -- "$1")
[[ $("$VELNIX_TEST_MKCERT" -version) == v1.4.4 ]] || { echo 'Expected mkcert v1.4.4' >&2; exit 2; }
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check
