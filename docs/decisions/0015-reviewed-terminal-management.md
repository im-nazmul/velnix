# ADR 0015 — Reviewed terminal management and session-owned workloads

Status: COMPLETE within the approved scope; see
[completion evidence](../validation/PHASE-5B-COMPLETION.md). Implemented under
direct user authorization: “Let do it. you dont need
show me the plan, you can go strait to the implementation”. This extends ADR
0014; its original read-only completion remains a historical bounded milestone.

## Scope

Add typed, allowlisted application operations for explicit private-state init,
project link/unlink, direct existing PHP/Node/Bun/Python selection/clear,
owned Nginx start/stop/reload and per-project FPM start/stop. Existing HTTPS
material can be used on an explicitly configured loopback listener. There is
no new runtime/provider/package install, systemd/database control, trust/DNS
modification, TLS issuance, project code execution, or automatic repair.

The user authorizes implementation without another proposal approval. This does
not bypass the per-operation preview/confirmation at runtime.

## Decisions

`app.TerminalSession` delegates to existing StateService, ProjectService,
RuntimeService and HTTPService. TerminalChange is data, never shell text. A
TerminalPlan retains the exact request privately and exposes its approval ID
and review data. Apply passes that request and ID to the existing service;
identity, registry, metadata, configuration and executable checks remain there.
The review includes request, affected target, effects, limitations and plan ID.
Unrelated registry targets are omitted; reviews beyond the display budget fail
rather than allowing approval of silently truncated data.

The action picker launches guided forms. Submission only creates a preview.
Approval is disabled until the review reaches its end, then requires typing
APPLY and Enter. Editing/cancellation discards the preview; late results cannot
restore it. Apply failures never automatically retry or generate a newly approved
plan. The operation result is visible before returning to the workspace.

Foreground Nginx/FPM Apply calls remain alive under the session context. Only
readiness of the exact reviewed running generation returns control to the UI.
The session tracks foreground owners, cancels and waits for cleanup on exit,
and reports lifecycle/cleanup failures. Closing an observing session never
signals other owners. Explicit reviewed stop/reload may affect existing owned
workloads as in the CLI. No detached launcher or daemon is introduced.

FPM reload is unsupported by the existing backend, so the UI offers separate
reviewed stop and start; it does not hide an automatic restart behind reload.
Interruption can leave recovery-required state under existing journal rules;
recovery stays an explicit CLI workflow. Unlink does not automatically stop web
routing, and runtime selection's external version probe is disclosed before
approval. The interface never sources a shell or auto-detects approval.

## Tradeoffs

Session-owned workloads stop on TUI exit. Users needing an independent lifetime
can use the existing foreground CLI in another terminal. Read-only discovery and
inspection still cause no mutations; management is available through explicit
actions only. Existing filesystem-call cancellation and external-runtime pinning
limits remain. No new registry schema is introduced.

Session owners are tracked by workload identity. A successful explicit stop waits
for its session owner to finish cleanup before a new generation may start. This
avoids old cleanup racing with an immediately approved restart.
