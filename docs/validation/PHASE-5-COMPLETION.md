# Phase 5 completion

**Status: COMPLETE — approved read-only implementation scope, 2026-11-05.**

The bounded [proposal](../PHASE-5-PROPOSAL.md) was approved by “Lets begun
implementation”. Subsequent user feedback established the welcoming home and
searchable-action experience. The request “Continue work and finish up phase 5”
authorizes this closeout. No implementation item remains pending within that
scope. Platform/release acceptance remains separate.

## Completion checklist

| Approved item | Delivered and verified |
|---|---|
| Optional Bubble Tea/Lip Gloss interface | `velnix tui`; pinned dependencies; no daemon or schema changes |
| Welcoming entry and action discovery | Centered home, evidence-based summary, searchable Ctrl+P actions, numbered destinations |
| Overview | Project count, owned HTTP/TLS and per-project FPM status, configuration/platform findings |
| Projects | Filter, paths/domains/selections, bounded metadata and owned web/FPM details |
| Runtimes | Passive candidates/provider capabilities and stored selection evidence; no executable probes |
| Services | Owned HTTP/FPM/TLS results; explicit unsupported database/system-service inspection |
| Diagnostics | Existing doctor/configuration findings, local errors and CLI next steps; no repair |
| Shared application boundary | `TerminalQuery` reuses application services; transport does not parse CLI output |
| Keyboard and responsive presentation | Lists, preview, readable/technical detail modes, filtering, refresh, paging, help and short-terminal action picker |
| Cancellation and stale data | Generation-checked async reads, cancellation/deadlines, explicit refresh, no polling |
| State preservation | Absent/initialized/corrupt-state tests and PTY configuration-error scenario preserve contents/permissions |
| Terminal safety | Display sanitization/caps, interactive-I/O requirement, terminal mode/alternate-screen restoration |
| Docs and automation | Usage, ADR 0014, acceptance script and CI terminal step |

## Final verification

Locally passed:

```sh
go test -race -count=1 ./...
go vet ./...
go mod verify
go build -trimpath -o /tmp/velnix-phase5 ./cmd/velnix
python3 scripts/validate-tui.py /tmp/velnix-phase5
git diff --check
git diff --cached --check
```

The full race suite includes existing real local HTTP/FPM integrations and ran
with permission to open disposable sockets. PTY scenarios cover q, Ctrl-C,
SIGTERM and invalid configuration; all restore terminal attributes/alternate
screen and preserve disposable XDG state. The invalid configuration remains
visible as a problem rather than being repaired or silently replaced. The
script also exercises home, palette search, all five inspection screens,
refresh/details/filter/help and resize. Further evidence is in
[PHASE-5-TUI](PHASE-5-TUI.md).

CI now invokes the same PTY script after building the binary. This records local
validation of that script, not an unobserved hosted CI result. User feedback
accepted the welcoming interface skeleton; PTY/model checks do not establish
visual correctness in every terminal emulator.

## Retained boundaries

- Read-only milestone: interactive mutations/probes, system database adapters,
  runtime installs, mail and dashboard are outside scope.
- No host package/service/DNS/trust modification was performed.
- Existing filesystem APIs cannot forcibly interrupt a blocked kernel read;
  contexts are checked between application reads and stale results are rejected.
- Native Ubuntu/non-systemd and additional terminal/Windows environments remain
  unperformed acceptance; see [RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md).
- Phase 6 is the next roadmap milestone. Its bounded proposal and implementation
  require separate authorization; Phase 5 completion does not start it.
