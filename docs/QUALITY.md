# Formatting and linting

Run repository checks from the repository root. Prerequisites are Git, the Go
version in `go.mod`, Bun from `.bun-version`, and Python 3.12 or newer. The pinned
native tool bootstrap currently supports Linux x86_64, including Ubuntu on WSL2;
PowerShell scripts are analyzed there without executing their target operations.

```sh
bun install --cwd web --frozen-lockfile
bun run --cwd web build # generate assets before Go checks
bun run tools:install
bun run check
```

`tools:install` explicitly downloads checksum-verified native tools into ignored
`.tools/` and warms the pinned Go tools in Go's normal module/build caches. It
requires network access on first installation and does not install system or
global tools. Remove `.tools/` to remove the local native tool installation.
Bun dependencies stay in `web/node_modules`; the root package only dispatches
commands, so `web/bun.lock` remains the dependency lockfile.

## Commands

| Command                  | Purpose                                                                                    |
| ------------------------ | ------------------------------------------------------------------------------------------ |
| `bun run format`         | Format all supported repository source and documentation                                   |
| `bun run format:check`   | Check formatting without rewriting files                                                   |
| `bun run lint`           | Run all language linters without automatic fixes                                           |
| `bun run lint:fix`       | Apply supported ESLint, Stylelint, Ruff and Markdown fixes, then check remaining languages |
| `bun run check`          | Run formatting checks, lint and Vue/TypeScript type checks                                 |
| `bun run --cwd web lint` | Run only dashboard ESLint and Stylelint                                                    |

After automatic lint fixes, run `bun run format` and `bun run check` again.
Lint checks fail on warnings as well as errors. Linting does not replace the
existing Go, browser and runtime acceptance tests.

## Language coverage

| Files                          | Formatter                                | Linter/checker                                                                 |
| ------------------------------ | ---------------------------------------- | ------------------------------------------------------------------------------ |
| Go                             | `gofmt -s` from the project Go toolchain | `go vet`, Staticcheck                                                          |
| Vue single-file components     | Prettier                                 | ESLint Vue recommended rules, typescript-eslint, vue-tsc; Stylelint for styles |
| TypeScript/JavaScript          | Prettier                                 | ESLint, typescript-eslint, TypeScript (including Vite/Playwright tooling)      |
| CSS                            | Prettier                                 | Stylelint standard rules                                                       |
| Python                         | Ruff formatter                           | Ruff syntax, correctness and import rules                                      |
| Shell                          | shfmt                                    | ShellCheck                                                                     |
| PowerShell and module settings | PSScriptAnalyzer `Invoke-Formatter`      | PSScriptAnalyzer warnings and errors                                           |
| Markdown                       | Prettier                                 | markdownlint-cli2                                                              |
| TOML                           | Taplo                                    | Taplo syntax validation                                                        |
| JSON/JSONC, YAML, HTML and SVG | Prettier                                 | Prettier parsing                                                               |

Editor whitespace defaults live in `.editorconfig`; optional VS Code extension
recommendations live in `.vscode/extensions.json`. ESLint disables formatting
rules that conflict with Prettier. Stylelint's descending-specificity heuristic is
disabled because the current stylesheet uses disjoint component selectors with
shared element names; changing their order can change the cascade. Markdown
allows long lines and existing inline HTML. Taplo checks syntax without fetching
external schemas.

## Go formatting and lint fixes

All repository Go files, including tests, are formatted and checked with
`gofmt -s`. This applies canonical Go layout and the formatter's supported
syntax simplifications. `go vet` and Staticcheck separately analyze `cmd/` and
`internal/` for correctness issues. Formatting does not apply every lint fix or
rewrite all equivalent-looking declarations.

For example, `lines := []string{}` in `friendlyView` was replaced with
`var lines []string` because both branches assign a new slice before reading the
initial value. That initial assignment was unused. It was a specific lint fix,
not a formatter rule requiring every empty slice to become nil.

Prefer the zero-value declaration for new local slice accumulators when nil and
empty are interchangeable. Preserve intentional non-nil empty slices at API,
JSON and comparison boundaries: a nil slice marshals to `null`, an initialized
empty slice marshals to `[]`, and equality checks can distinguish them. Review
such changes as code changes rather than applying a repository-wide replacement.
See the [Go empty-slice guidance](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices)
and [gofmt options](https://pkg.go.dev/cmd/gofmt).

## Scope and version updates

`scripts/quality.py` enumerates tracked and untracked, non-ignored repository
files. Generated dashboard assets, dependency directories, local tools and test
artifacts are excluded. Vue templates and scripts are formatted with the same
Prettier configuration as other frontend source files. License notices are preserved. Regenerate embedded
assets with `sh scripts/build.sh` after frontend changes; generated output is
checked separately for reproducibility and size in CI. Go analysis and CI tests
target `cmd/` and `internal/` explicitly so Go examples shipped inside JavaScript
dependencies are excluded.

JavaScript tools use exact versions in `web/package.json` and `web/bun.lock`.
Native download URLs, versions and SHA-256 digests, plus Go tool module versions,
are pinned in `tools/quality.lock.json`. Update those together and rerun install,
formatting checks, lint, type checks and the relevant application tests. Native
bootstrap downloads are currently Linux x86_64 only; running the full suite on
other architectures requires adding reviewed platform artifacts first.

CI performs the same frozen dependency installation, explicit tool installation
and `bun run check` before asset and application acceptance checks.
