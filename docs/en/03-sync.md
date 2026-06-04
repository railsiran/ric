# `ric sync` — keep host and container in sync

`ric` keeps the project on your host (so your editor, your git, your linter can see it) AND inside a Docker container (so Rails, the gems, and the build tools can run it). `ric sync` is how you move files between the two.

## Usage

```bash
# host → container (default)
ric sync

# container → host
ric sync --reverse
```

You must run `ric sync` from inside the project directory created by `ric new`. The directory name is also the container name.

## Direction

- **`ric sync`** copies the files in your project directory **into the container**. Use this after editing code on the host (e.g. you saved a controller in VS Code) so the container sees the changes.
- **`ric sync --reverse`** copies files **from the container back to the host**. Use this after a generator or any command run inside the container that produced new files (a migration, a new model, a generated test).

## What gets skipped

`ric sync` deliberately ignores two directories:

- `tmp/` — caches, PIDs, and other ephemera that have no business crossing the boundary
- `storage/` — Active Storage files and your development SQLite databases. These belong to the container's filesystem and should not be overwritten by whatever is (or isn't) on the host.

## How it works (briefly)

For each top-level entry that isn't `tmp/` or `storage/`, ric removes the destination copy and runs `docker cp -a` to replace it. It's a coarse-grained sync — fast, simple, and predictable. Not the right tool for sub-file diffs, but exactly what you want for "make these two trees match".

## When you'd run it

- After installing a new gem (edit `Gemfile` on host → `ric sync` → `ric rails -- bundle install`)
- After running a Rails generator inside the container (`ric rails -- generate model …` → `ric sync --reverse`)
- Before `ric deploy` if you want to make sure the container has your latest edits

(Most ric commands that run things inside the container — like `ric rails` — already sync changes back to the host after they finish, so you typically only need `ric sync` for host→container.)

## What's next

- Run a Rails command inside the container: [`ric rails`](05-rails.md).
- Open a console: [`ric console`](04-console.md).
