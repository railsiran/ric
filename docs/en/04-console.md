# `ric console` — interactive Rails console

`ric console` drops you into `bin/rails console` inside the project's container. Same Ruby, same gems, same database as the container — just an interactive REPL on top.

## Usage

```bash
# local dev container
ric console

# production console on the deployed server
ric console --remote
```

Run from inside the project directory. The container name is the directory name (created by `ric new`).

## Local mode (no flag)

Opens a Rails console inside the local dev container. The console talks to your development SQLite database under `storage/`. Hit `exit` or `Ctrl+D` to leave.

```bash
$ ric console
Opening Rails console for myapp...
Loading development environment (Rails 8.0.x)
irb(main):001> User.count
=> 0
```

## Remote mode (`--remote`)

Opens a Rails console **inside the production container on your deployed server**. Same SSH connection ric uses for deploys, multiplexed for speed. Reads `.ric/deploy.yml` for host, user, port, and key (or password).

```bash
$ ric console --remote
Opening production Rails console for myapp on 198.51.100.42...
Loading production environment (Rails 8.0.x)
irb(main):001> User.last.email
=> "real-customer@example.com"
```

### What you need

- The project must already be deployed with [`ric deploy`](06-deploy.md).
- `.ric/deploy.yml` must be in the current directory.
- Your SSH user on the server must be root, or have passwordless sudo (ric set this up for you on first deploy).

### Be careful

A remote console is talking to **real production data**. Treat it like ssh-ing into prod:

- Avoid destructive operations unless you're sure.
- `Model.delete_all` is forever — there's no undo.
- Long-running queries hold a connection from your app's pool.

When you're done, `exit` or `Ctrl+D` — the SSH master connection tears itself down on exit, so there are no leftover sessions on the server.

## What's next

- Run a one-shot Rails command instead of an interactive session: [`ric rails`](05-rails.md).
- Deploy or upgrade: [`ric deploy`](06-deploy.md), [`ric upgrade`](07-upgrade.md).
