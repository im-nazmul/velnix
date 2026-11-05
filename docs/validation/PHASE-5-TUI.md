# Phase 5 read-only TUI validation

Date: 2026-11-05. Status: COMPLETE within the approved scope; see
[completion record](PHASE-5-COMPLETION.md). Locally validated within the approved
[proposal](../PHASE-5-PROPOSAL.md). This is not release-wide platform acceptance.

## Delivered

`velnix tui` presents overview, projects, runtimes, owned services and diagnostics
through application queries. It supports keyboard selection, name filtering,
details, scrolling, explicit refresh, help and terminal resize. Queries do not
initialize/repair state, probe executables, invoke managers or perform mutations.
See [usage and limits](../TUI.md) and [ADR 0014](../decisions/0014-read-only-terminal-interface.md).

## Local evidence

- Full `go test -race ./...` passed, including existing real HTTP/FPM/runtime
  integration tests, with permission to create disposable local sockets. The
  initial sandboxed full test run failed on denied socket operations; those
  failures were environmental and were rerun with socket access.
- Focused terminal application/CLI/model tests passed after final refinements.
  They cover canceled and stale reads, filtering/detail navigation, narrow
  terminal bounds, display sanitization/output caps, rejection of script I/O and
  JSON, corrupt configuration/registry errors, partial capability results and
  registered-project metadata/profile inspection.
- Every screen was queried twice against absent and initialized disposable
  state; file contents and permissions remained unchanged. Corrupt registry
  content also remained unchanged after failed reads.
- `go vet ./...`, build and `git diff --check` passed.
- Linux PTY acceptance exercised all five screen results, detail/refresh/help,
  filtering, resize from 100x24 to 24x8, normal q, Ctrl-C and SIGTERM. Terminal
  attributes and alternate-screen cleanup were restored and no XDG state was
  created. Normal q and Ctrl-C returned 0; SIGTERM returned the CLI cancellation
  code 130. The initial SIGTERM test exposed competing signal handlers; disabling
  Bubble Tea's extra handler fixed the shutdown race and repeat runs passed.

Reproduce the PTY check with Python 3 on Linux:

```sh
go build -o /tmp/velnix-tui ./cmd/velnix
python3 scripts/validate-tui.py /tmp/velnix-tui
```

The PTY check uses temporary XDG locations and sends keystrokes/signals only to
its child binary. It does not modify host services, DNS or trust.

## Boundaries

PTY evidence is local Linux/WSL terminal emulation, not visual acceptance in every
terminal emulator or native Ubuntu/Windows terminal validation. No claim is made
for those environments. Missing service records display “Not configured”; unavailable reads
carry reasons. System database inspection/lifecycle remains explicitly unsupported.

Context deadlines do not forcibly interrupt blocked kernel filesystem reads;
queries check cancellation between existing application reads. Large detail text
is truncated for display. No polling, interactive mutations, runtime probing,
mail, dashboard or host integration was added. Existing
[release acceptance](RELEASE-ACCEPTANCE.md) gates remain open.

## Presentation refinement

The read-only interface now uses compact navigation, a context-sensitive footer,
aligned summary columns, readable default details and an optional technical view.
Wide terminals show a selection preview; smaller sizes retain the single-list
interaction. Rendering tests cover both layouts and technical-data visibility.
Transport race tests, vet, build and the updated three-exit PTY acceptance check
were rerun for this refinement. Application queries and mutation boundaries are
unchanged.

Additional interaction refinement verifies selection preservation across
refresh/reordering, returning to a filtered list, detail paging without selection
changes, clamped scrolling and scrolling help without altering underlying details.
Status summaries include text markers so meaning survives disabled color.

## Welcoming home and action discovery

Startup now presents a spacious home screen rather than a diagnostic list.
Enter/Ctrl+P opens a searchable action picker; home shortcuts open the four
main destinations. The existing read-only application query boundary remains
unchanged. Tests exercise filtered action navigation, dismissing the picker,
and home rendering bounds at large and small sizes. PTY acceptance now also
checks the home wordmark and opens Projects through the searchable picker before
exercising existing screens and all exit paths.

Home summary refinement uses existing overview data only. Rendering tests verify
that absent projects differ from unavailable project reads and that a saved TLS
certificate never implies running/trusted HTTPS. Multiword action-search tests,
transport race tests, vet, build and PTY acceptance cover this addition.

Closeout adds per-project FPM status to Overview, usable short-terminal action
selection, full help scrolling and a PTY invalid-configuration scenario. The
acceptance script checks state contents/permissions in addition to terminal
restoration. CI runs this script; its hosted result remains unobserved locally.
