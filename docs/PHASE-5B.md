# Phase 5B — interactive terminal management

**Status: COMPLETE within the approved interactive-management scope.**
See [completion evidence](validation/PHASE-5B-COMPLETION.md). Implemented following
direct approval to proceed without presenting a proposal.
The original Phase 5 read-only milestone remains complete; this document records
its explicitly authorized extension, not authorization for Phase 6.

Press Ctrl+P and search for one of these actions:

| Action | Inputs and behavior |
|---|---|
| Initialize state | Explicit preview of private XDG state creation; never automatic |
| Link project | Name, absolute project path, document root; source stays external |
| Unlink project | Name; removes registry entry, preserves source |
| Select runtime | Project, php/node/bun/python, existing absolute executable, exact version; approval includes the version probe |
| Clear runtime | Project and kind; removes only its stored selection |
| Start web | Installed Nginx, HTTP port, optional existing HTTPS certificate/port |
| Stop web / Reload web | Existing owned workload and current routes |
| Start PHP pool | Project, installed FPM executable, exact version, profile and upload directories |
| Stop PHP pool | Project; existing backend does not support reload |

The focused project is prefilled when opening an action from Projects. Enter or
Tab advances the form; Shift-Tab goes back, Ctrl-U clears a default, Backspace
edits, and Escape cancels. Paths with spaces are accepted literally. Fields are
passed as typed data to application services, never interpolated into shell text.

Management requires at least 40 columns and 12 rows; smaller terminals show a
resize prompt and disable approval. Escape can still cancel.

Submitting a form produces a preview only. Scroll with arrows/PageUp/PageDown,
or End to reach the end. Read the requested inputs, effects, affected target,
limitations and plan ID. Type APPLY and press Enter to execute that exact plan.
Changes since preview cause refusal and require a new review. Apply never
silently updates approval or retries. Escape cancels a preview, and Ctrl-C exits
and cancels the session. While applying, other management/navigation actions
are blocked; interrupted operations may require existing CLI recovery.

Nginx/FPM started here belong to this TUI session and stop when it exits. The
review states this before approval. Workloads started by another foreground
owner keep their independent lifetime when this TUI closes. Explicit approved
stop/reload still affects the selected owned workload. Cleanup errors are shown
in session diagnostics and returned on exit.

Unlink requires a separate reviewed web reload/stop to revoke existing routes.
Stopping FPM can interrupt PHP requests. FPM changes use separate stop/start
reviews; no automatic restart is implemented. HTTPS startup requires previously
issued material; certificate issuance/trust/DNS operations remain outside scope.
No package install, manager command activation, recovery automation, shell,
system service or database lifecycle, mail or dashboard was added.

Implementation boundaries and tradeoffs: [ADR 0015](decisions/0015-reviewed-terminal-management.md).

Local checks and reproducible acceptance: [validation](validation/PHASE-5B-MANAGEMENT.md).

A successful stop waits for this session’s foreground owner cleanup before an
immediate restart. Apply errors wrap and scroll with arrows/PageUp/PageDown or
Home/End; Enter returns to the workspace without retrying the operation.
