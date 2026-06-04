# `ric rails` — run any Rails command in the container

`ric rails -- <command>` runs `bin/rails <command>` inside the project's container, with stdin/stdout/stderr wired through to your terminal. After it finishes, the result is copied back from the container to the host so you immediately see new files (generators, migrations, etc.) in your editor.

## Usage

```bash
ric rails -- generate model Post title:string body:text
ric rails -- db:migrate
ric rails -- db:rollback
ric rails -- routes
ric rails -- runner "puts User.count"
```

Run from inside the project directory.

The literal `--` separates `ric`'s own flags from the arguments forwarded to `rails`. Everything after `--` is passed verbatim.

## What it does

1. Verifies the container `<name>` is running.
2. Executes `docker exec -w /workspace/<name> <name> bin/rails <your args>` with stdin/stdout/stderr attached.
3. After the command completes, removes everything in the host project directory (preserving `.git/` if present) and re-copies the entire `/workspace/<name>` tree from the container.

That last step is the key feature: anything the Rails command generated (new model files, controllers, migrations, schema.rb updates) lands on your host without you having to run `ric sync --reverse` manually.

## Common uses

- **Generators.** `ric rails -- generate scaffold Post title body:text` — creates the migration, model, controller, views, tests, and you'll see them in your editor immediately.
- **Migrations.** `ric rails -- db:migrate`, `ric rails -- db:rollback`, `ric rails -- db:seed`.
- **Routes/inspection.** `ric rails -- routes` for the full route table.
- **Quick scripts.** `ric rails -- runner "puts User.count"` to run a Ruby snippet against your app's environment.

## Differences vs `ric console`

| | `ric console` | `ric rails -- …` |
|---|---|---|
| Interactive REPL | yes | no |
| One-shot command | no | yes |
| Auto-sync back to host | n/a | yes |
| Best for | exploration | generators, migrations, scripted tasks |

## Caveats

- The host project directory is rewritten on every run. If you have unstaged changes that you also wrote inside the container during the same session, the host copy is what wins for `.git/` and the container's copy wins for everything else.
- Be careful with commands that write data to `storage/` from a runner — that's fine, but understand the file goes into the container's filesystem first and is then copied to the host.

## What's next

- For sync without running anything: [`ric sync`](03-sync.md).
- For an interactive console: [`ric console`](04-console.md).
