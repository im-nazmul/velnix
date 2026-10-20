# Phase 2 project registry

Phase 2 was authorized by “Lets start work with phase 2”. This first slice adds durable project metadata and commands. Rootless Nginx, HTTP serving and process lifecycle are still pending; Phase 2 is not complete.

## Usage

Initialize private state first using the reviewed `state init` plan described in [FOUNDATION](FOUNDATION.md). Flags follow the leaf command:

```sh
velnix projects list --json
velnix projects link --name demo --path /absolute/project --document-root public --dry-run --json
# Review the complete plan, including registry upgrade and target metadata:
velnix projects link --name demo --path /absolute/project --document-root public --approve PLAN_ID --json
velnix projects inspect --name demo --json
velnix projects unlink --name demo --dry-run --json
velnix projects unlink --name demo --approve UNLINK_PLAN_ID --json
```

`--path` defaults to the current directory; `--document-root` defaults to the project root. Relative document roots resolve against the project root. An absolute document root is accepted only inside that root after resolving symlinks. The explicit name must be a lowercase DNS label of 1–63 characters. The reserved domain is `NAME.test`; aliases and custom domains are deferred. Linking does not enable an HTTP listener or change hosts files.

List and inspect read stored metadata without scanning source files. An uninitialized list returns `[]`; inspect of an absent project returns unavailable. Link/unlink require initialized, healthy state. Plain output is formatted JSON; `--json` provides one compact JSON value with the existing stderr error envelope and exit codes. No command prompts.

## Safety and recovery

Preview is the default for mutations; `--dry-run` and `--approve` cannot be combined. Approval covers the current registry, journal, canonical paths and directory device/inode identities. A stale plan fails and must be previewed again. Names and canonical roots are unique; an existing identical link or absent unlink is a no-op. Changing linked metadata requires an explicit unlink/link sequence.

Project source and data remain external and are never written or deleted. Unlink works even if the project directory has moved or disappeared. Metadata is capped at 1000 projects and the existing private state file limit.

The first effective mutation upgrades registry schema 1 to 2, retaining resources. Existing schema-1 initialization remains supported. Older Velnix binaries cannot read the resulting state; no automatic downgrade is provided. See [ADR 0008](decisions/0008-project-registry-transactions.md).

One journal retains the latest transaction. Project operations share the initialization mutation lock and block on unresolved operations. After a failed/interrupted mutation, inspect `state status`, then preview `state recover --dry-run --json` and approve its exact plan. Recovery marks unchanged before-state failed or committed target-state complete. It never replays a write. Unknown external edits are preserved for manual review.

Directory identity checks describe observations at link time; they do not freeze source paths or make them safe for serving. The upcoming HTTP slice must enforce its own path and content policy.
