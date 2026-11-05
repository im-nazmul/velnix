# ADR 0014 — Read-only terminal interface

Status: implemented; Phase 5 COMPLETE within its approved read-only scope.
Accepted by “Lets begun implementation”, following approval of the bounded
proposal. See [completion evidence](../validation/PHASE-5-COMPLETION.md).

Use pinned Bubble Tea/Lip Gloss v1 dependencies for an optional `velnix tui`.
Presentation lives in internal/transport/tui and reads presentation-neutral
inspection entries through internal/app.TerminalQuery. Domain values are
serialized into inspectable detail text at the application query boundary;
CLI text is never parsed. Existing application services remain authoritative.

The first milestone has no mutation or probe controls. Missing state is not
initialized, detected installations remain external, and capability reporting
never grants ownership. System database status/lifecycle adapters are deferred.

Refresh and navigation create cancellable queries with generation IDs. Late
results cannot replace current results. Reads have context deadlines and no
continuous polling; existing non-context filesystem reads cannot be forcibly
interrupted. Keep failures local to each panel, sanitize untrusted display data,
and restore terminal modes on exit. The CLI owns process signals; Bubble Tea's
additional signal handler is disabled to avoid a shutdown race.

This adds terminal dependency and maintenance costs, but does not add a daemon,
registry migration, privilege boundary or host mutation. Interactive mutations
need separate scope and exact-plan confirmation. Native Ubuntu, other terminals
and existing release acceptance remain distinct validation gates.
