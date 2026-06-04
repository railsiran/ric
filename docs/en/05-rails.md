# `ric rails` — run any Rails command in the container

`ric rails -- <command>` runs `bin/rails <command>` inside the project's container, with stdin, stdout, and stderr wired through to your terminal. After it finishes, the result is copied back from the container to the host so you immediately see new files (generators, migrations, etc.) in your editor.

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

First, ric verifies the container `<name>` is running.

Second, it executes `docker exec -w /workspace/<name> <name> bin/rails <your args>` with stdin, stdout, and stderr attached to your terminal, so generators that print and prompt work normally.

Third, after the command completes, ric removes everything in the host project directory (preserving `.git/` if present) and re-copies the entire `/workspace/<name>` tree from the container.

That last step is the key feature: anything the Rails command generated — a new model, a controller, a migration, an update to `schema.rb` — lands on your host without you having to run `ric sync --reverse` manually.

## Common uses

**Generators.** Run something like `ric rails -- generate scaffold Post title body:text`. This creates the migration, model, controller, views, and tests inside the container, and you'll see them in your editor a moment later.

**Migrations.** `ric rails -- db:migrate` runs pending migrations. `ric rails -- db:rollback` rolls back the most recent. `ric rails -- db:seed` runs the seed file.

**Routes and inspection.** `ric rails -- routes` prints the full route table. `ric rails -- about` prints version info for Rails and its dependencies.

**Quick scripts.** `ric rails -- runner "puts User.count"` runs a Ruby snippet against your app's environment, with all your models loaded.

## Differences vs `ric console`

`ric rails -- <command>` is for one-shot operations and always syncs the container's filesystem back to the host when it finishes. It's the right choice for generators, migrations, and scripted tasks.

`ric console`, on the other hand, opens an interactive REPL that does not sync files back. You stay in the console until you `exit`. It's the right choice for exploration: poking at models, trying queries, debugging records.

So: use `ric console` when you want to talk to the app interactively, and use `ric rails -- ...` when you want to run a single command and have its file output appear in your editor.

## Caveats

The host project directory is rewritten on every run. If you have unsaved changes that you also wrote inside the container during the same session, the host copy wins for `.git/` and the container's copy wins for everything else.

Be careful with commands that write data to `storage/` from a runner script. The writes succeed, but understand the file lands in the container's filesystem first and is then copied to the host on sync-back.

## What's next

For sync without running anything, see [`ric sync`](03-sync.md).

For an interactive console, see [`ric console`](04-console.md).
