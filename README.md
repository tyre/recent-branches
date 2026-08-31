# recent-branches

A terminal UI for switching between your recent git branches. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![recent-branches Demo](https://github.com/user-attachments/assets/b0966d8f-9d8e-4c40-a7af-fc98d9e2d342)

It lists your most recently active branches in a table, shows the last few commits on whichever branch you highlight, and switches when you hit enter. If you have uncommitted changes, a modal asks whether to commit or stash them first.

Supports diffing to see what's on those dusty old branches

![Diff preview](https://github.com/user-attachments/assets/ae0f3bbf-239a-4e7f-bbd2-5e7dc42714be)

With per-file view

![Diff file wahooooo](https://github.com/user-attachments/assets/43e12e23-416e-41cf-aff6-3ad52d9db4dd)


## Install

Requires Go 1.24+.

```sh
make build        # builds to build/recent-branches
# or
go build -o recent-branches
```

## Usage

Run it from inside a git repository:

```sh
recent-branches                  # your 10 most recent local branches
recent-branches -n 20            # show more
recent-branches -remote          # include remote branches
recent-branches -author all      # everyone's branches
recent-branches -author a,b      # specific authors
recent-branches -debug           # enable debug logging and the logs panel
```

By default it shows only branches you have committed to.

### Keys

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move through branches (or scroll logs when focused) |
| `enter` | Switch to the selected branch |
| `d` | Diff the selected branch against the current branch |
| `tab` | Toggle focus between table and debug logs (requires `-debug`) |
| `r` | Refresh the branch list |
| `l` | Clear debug logs (requires `-debug`) |
| `c` | Clear the status message |
| `q` / `ctrl+c` | Quit |

### Diff view

Press `d` on a highlighted branch to see what it changes relative to the
current branch (three-dot diff, so: commits on that branch since it diverged).
A summary line shows total files and +/- counts. Files start collapsed with
their own +/- counts; move with `↑`/`↓`, expand one with `space`/`enter`.
Press `b` to pick a different base branch, `esc`/`q` to go back.

## Development

```sh
make dev     # quick build in the current directory
make test    # go test -v ./...
make fmt     # go fmt
make lint    # golangci-lint
```
