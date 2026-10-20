# ADR 0008 — Project registry transactions

- Status: Accepted for the first Phase 2 implementation slice
- Date: 2026-10-20
- Authorization: user instruction “Lets start work with phase 2”

## Decision

Start Phase 2 with explicit project list/inspect/link/unlink commands. A link owns registry metadata only; project directories remain external. Resolve project/document roots to canonical paths, constrain document roots to the project, reject duplicate names/roots, and bind link approval to directory device/inode identities. Directory metadata reads execute no project code. Names are lowercase DNS labels and reserve their exact `NAME.test` domain; HTTP and DNS are not configured by a registry link.

Use the existing private registry, mutation lock, durable replacement and plan approval contracts. Keep schema-1 registries readable. The first effective project mutation upgrades the registry to schema 2 in the approved transaction, preserving resource records. Projects are an optional array; omission represents an empty collection. Schema-1 initialization bytes remain unchanged. Downgrade is unsupported; older binaries refuse the new schema or operation journal. Preview includes the target registry and the upgrade effect.

The journal envelope remains version 1 and adds an optional before digest for `project-link`/`project-unlink`. Unknown operations are refused. Retain exactly the latest operation journal: replace it only after a terminal operation has matching registry evidence. Unresolved operations block project reads and mutations. No unbounded history or project-content backups are introduced.

Recover by recognizing the exact before/after registry bytes: before means failed with no replay; after means complete. Unknown edits require manual review. Record the journal before the atomic registry replacement. Cancellation or failure can leave an unresolved journal; recovery uses the same explicit plan approval. Recovering does not write project files or replay a registry operation.

## Consequences and limits

Registry transactions use Linux descriptor operations and the shared per-user flock. Canonical paths and device/inode checks detect observed directory replacements but are not a filesystem sandbox; same-user hostile races and changes after linking remain possible. Future HTTP operations must revalidate live paths and enforce serving restrictions independently.

Schema evolution is explicit and tied to approval. No automatic downgrade or historical undo is provided; unlink reverses a link through a new reviewed transaction. Registry size remains bounded by the existing file limit, with at most 1000 projects.

This slice does not accept pending Nginx/FPM/trust process designs by implication. Rootless HTTP and its process identity/failure gates remain the next Phase 2 work. PHP, runtimes and HTTPS remain Phase 3.
