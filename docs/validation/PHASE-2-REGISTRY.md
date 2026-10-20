# Phase 2 project registry validation

Date: 2026-10-20. Scope: the first Phase 2 slice in [PROJECTS](../PROJECTS.md), not the complete project/HTTP milestone. Local Go toolchain: go1.22.2 linux/amd64.

## Checks run

- `go test -race ./...` — passed all packages.
- `go vet ./...` — passed.
- `git diff --check` — passed.

The sandbox could not access the normal Go cache and initially reported a standard-library lookup failure. Tests and vet completed outside the sandbox with explicit tool approval. The first functional run identified the old future-schema fixture using schema 2; that fixture now uses unsupported schema 3 because this slice implements schema 2.

## Evidence covered by tests

- Read-only project list creates no state. Link preview leaves registry bytes unchanged; explicit matching approval is required.
- CLI service flow initializes state, previews/applies link, inspects canonical metadata, then previews/applies unlink. JSON output and external source preservation are checked.
- Project/document roots must be directories; document roots escaping through parent paths or symlinks are rejected. Directory replacement changes observed identity and invalidates a link approval.
- Name/root conflicts fail. Repeating an identical link is a no-op. Unlink works after a source directory moves and preserves that directory.
- A committed registry change invalidates another pending approval. Concurrent approvals serialize through the shared lock; exactly one original plan commits and no update is lost.
- Simulated interrupted project operations at before/after registry publication recover without replay. Unknown external registry bytes are preserved and require manual review. Unresolved operations block new project mutations.
- Existing foundation tests still pass, including process lock release/cancellation and unsafe state-path rejection.

## Pending evidence

No HTTP routing, Nginx lifecycle/config validation, occupied-port handling or process identity behavior is implemented or validated by this slice. The installed Nginx binary was observed as `/usr/sbin/nginx`, version 1.24.0; no Nginx workload was started. No Windows browser/networking matrix or native Ubuntu/WSL cross-platform validation was performed. Simulated journal boundaries are not power-loss/filesystem fault evidence. Phase 2 exit gates remain open.
