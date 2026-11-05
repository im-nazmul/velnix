# Terminal interface

Run `velnix tui` in an interactive Linux terminal. Input and output must both be
terminals; use existing CLI queries for scripts and JSON. No separate daemon or
state initialization is required.

The interface opens on a welcoming home screen with a centered wordmark and
four destinations: Projects, Runtimes, Services and Environment. Press Enter or
Ctrl+P to search available actions; type an action name and press Enter to open
it. Number keys 1–4 open the corresponding destination from home. Ctrl+P works
throughout the interface, including details and help. Escape closes the picker
without changing the underlying screen. Escape from an unfiltered list returns
home. This is an action picker, not a shell or AI prompt: it only opens existing
read-only views, refreshes data or displays help.

The inspection screens are Overview, Projects, Runtimes, Services and Diagnostics.
The interface uses compact screen tabs, aligned name/status columns and a
preview pane in terminals at least 92 columns wide with enough rows. Narrower
terminals use a single list. Select a row and press Enter for readable details;
press `t` to toggle the original technical data. Runtime candidates are passive
filesystem discoveries, not verified executable versions. Project selections
are stored evidence, not a fresh executable probe. Project details include
bounded framework hints and owned FPM/web profile status. Missing workloads show “Not configured”; failures include an unavailable reason.

| Key | Action |
|---|---|
| Enter on home / Ctrl+P anywhere | Search actions |
| 1–4 on home | Open a destination |
| Tab, right arrow, l | Next screen |
| Shift-Tab, left arrow, h | Previous screen |
| Up/down arrows, k/j | Select row or scroll details |
| PageUp/PageDown | Page rows or details |
| Home/End | First/last row or beginning/end of details |
| Enter | Open details; finish filter input |
| / | Filter row names; type text, Backspace to edit |
| Esc, Backspace | Back to filtered list; press again to clear filter |
| r | Refresh current screen |
| t | Toggle technical data in details |
| ? | Toggle help |
| q, Ctrl-C | Quit (while filtering, q is text; Ctrl-C always quits) |

Reads run asynchronously with a five-second context deadline. Navigation and
refresh cancel the previous context and reject its stale results. Existing
filesystem APIs cannot interrupt a blocked kernel filesystem call; cancellation
is checked between reads. There is no background polling. Long detail output is
capped for display; control/escape sequences are stripped. Narrow terminals clip
navigation text; details wrap, and arrow/letter keys still work. Rendering caps
at 500 columns and 200 rows to bound work.

The interface does not probe runtime binaries, execute repository code, repair
state, start/stop services or alter configuration. Database/system-service
inspection is explicitly unsupported in this milestone. Use the existing CLI
plan/approval workflows for mutations. Reading project hints uses the existing
bounded metadata reader and may report unavailable or changed project identity.

Bubble Tea v1.3.10 and Lip Gloss v1.1.0 are pinned to the compatible v1 APIs;
this milestone deliberately does not introduce the separate v2 module APIs.
Dependency checksums are committed in go.sum. Go's module solver also raises
x/sys to v0.36.0. Terminal signals use the existing CLI context handler to avoid
competing signal handling during shutdown.

Status markers remain meaningful without color: `✓` indicates a running or
healthy result, `~` highlights a warning/pending change, and `!` flags unavailable
or recovery/error states. Refresh preserves the selected row by name when it
still exists. Help is scrollable, and paging details never changes the selected
row. Detail footers show the visible line range.

The home summary uses the existing overview read: linked project count, owned
web process status and saved HTTPS certificate state. “Certificate saved” does
not imply a running HTTPS listener or browser trust. Missing or failed reads are
shown distinctly; a warning/error points to Environment with the `4` shortcut.
Action search accepts multiple words in any order.
