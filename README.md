# recent-branches

![recent-branches Demo](https://github-production-user-asset-6210df.s3.amazonaws.com/1015847/643321671-ba87bf49-3651-4a76-892b-22bdbaecc63a.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAVCODYLSA53PQK4ZA%2F20260831%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20260831T032252Z&X-Amz-Expires=300&X-Amz-Signature=36b6a1b16eff32edc9ec02bc4388d6d923ef0fdb4b4fab5903eaaaaa6eb15c78&X-Amz-SignedHeaders=host&response-content-type=image%2Fpng)


A terminal UI for switching between your recent git branches. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

It lists your most recently active branches in a table, shows the last few commits on whichever branch you highlight, and switches when you hit enter. If you have uncommitted changes, a modal asks whether to commit or stash them first.

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
