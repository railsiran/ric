# `ric deploy` — ship a project to your server

`ric deploy` takes the container you've been developing in and turns it into a running production service on a remote server — installing Docker if missing, generating production credentials, setting up a shared nginx with TLS, and starting the app. It's first-time-only; subsequent releases go through [`ric upgrade`](07-upgrade.md).

## Usage

```bash
# in the project directory
ric deploy
```

No flags, no arguments. Everything comes from `.ric/deploy.yml`.

## Set up `.ric/deploy.yml`

Create this file at `.ric/deploy.yml` in your project root. Minimal example:

```yaml
host: 198.51.100.42
user: root
domain: shop.example.com
ssl_dir: /etc/letsencrypt/live/shop.example.com
```

Full example with every option:

```yaml
# --- SSH connection ---
host: 198.51.100.42             # required: IP or hostname
user: root                      # required: SSH user (root, or has passwordless sudo)
port: 2222                      # optional: defaults to 22
ssh_key: ~/.ssh/id_ed25519      # optional: omit for password auth

# --- App / TLS ---
domain: shop.example.com            # required: nginx server_name
ssl_dir: ~/certs/shop.example.com   # required: local dir with fullchain.pem + privkey.pem

# --- Registry mirror (optional) ---
registry_mirror: dockerhub.example.com   # if your VPS can't reach Docker Hub directly
```

### Field reference

| Field | Required | Meaning |
|-------|----------|---------|
| `host` | yes | IP or hostname of the server |
| `user` | yes | SSH user — must be root or have passwordless sudo (ric will bootstrap it for you once if needed) |
| `ssh_key` | no | Path to your SSH private key. Omit for password auth — you'll be prompted once |
| `port` | no | SSH port (default 22) |
| `domain` | yes | Domain the app will be served at; goes into nginx `server_name` |
| `ssl_dir` | yes | **Local** directory containing your certificate as `fullchain.pem` + `privkey.pem` |
| `registry_mirror` | no | A Docker Hub pull-through mirror used by the daemon (useful where Docker Hub is filtered/slow) |

`~` is expanded in `ssh_key` and `ssl_dir`.

## What `ric deploy` does

1. **Validates locally** — reads `.ric/deploy.yml`, checks SSL files exist, verifies the dev container is running.
2. **Builds the image** — `RAILS_ENV=production bin/rails assets:precompile`, then `docker commit` + `docker save` to a content-addressed `.tar` (`<sha256>.tar`). The temporary local image is cleaned up.
3. **Opens an SSH master connection** — multiplexed so you enter the password (if any) at most once for the whole deploy.
4. **Detects remote privilege** — if your SSH user isn't root and lacks passwordless sudo, ric prompts for sudo once and writes `/etc/sudoers.d/ric-<user>` so future deploys are silent.
5. **Installs Docker** on the server if it isn't there.
6. **Configures the registry mirror** in `/etc/docker/daemon.json` and verifies it by pulling `hello-world`.
7. **Provisions the layout** at `/var/lib/ric/<project>/` (credentials, storage, images) and the shared `/var/lib/ric/_shared/` (sites-enabled, ssl, ric-nginx).
8. **Uploads the image tar**, SSL certificates, and a generated nginx site config.
9. **Generates production Rails credentials** on the server (`config/credentials/production.key` + `production.yml.enc`) and persists them; pulls the master key back to local `.ric/keys/production.key` (mode 0600) so you can recover it later.
10. **Runs `db:prepare`** (creates + migrates + seeds the Solid Trifecta databases under the persistent `storage/` mount).
11. **Starts the app container** with `bin/rails server -b 0.0.0.0 -p 3000` and `SOLID_QUEUE_IN_PUMA=true` (the queue worker runs in-process with web).
12. **Starts the shared `ric-nginx`** if it isn't already running, binds 80/443, then reloads it to pick up your site config.
13. **Writes `current_sha`** as the success marker.

Final line: `Deployed <name> @ <short_sha> to <host>. Live at https://<domain>`.

## After deploy

- `.ric/keys/production.key` exists on your machine — **add `.ric/keys/` to your `.gitignore`**.
- The remote layout under `/var/lib/ric/<name>/` and `/var/lib/ric/_shared/` is what subsequent `ric upgrade` runs build on.
- Want a production REPL? `ric console --remote` (see [`ric console`](04-console.md)).
- Want to push a new version? [`ric upgrade`](07-upgrade.md).

## Multiple projects on one server

`ric deploy` is designed for it. The first project to deploy starts a singleton `ric-nginx` and binds 80/443; subsequent projects just drop a site config under `/var/lib/ric/_shared/nginx/sites-enabled/<name>.conf` and `nginx -s reload` picks them up. Routing is by `server_name` (the `domain` you configured per project).

## Re-running deploy

If you run `ric deploy` and a previous deploy already completed (the `current_sha` marker exists), it refuses and points you at `ric upgrade`. That's intentional — `deploy` is provisioning; `upgrade` is releasing.

If a previous `ric deploy` *crashed midway*, the partial state is detected and cleaned up automatically on the next run.

## Troubleshooting

- **"the registry mirror is not serving …"** — your mirror value in `.ric/deploy.yml` is wrong (often a scheme like `https://` or a path is included). Check `/etc/docker/daemon.json` on the server.
- **App container in restart loop** — `ssh user@host docker logs <name>`. Common culprits: missing gem, migration failure, master key issue.
- **nginx in restart loop** — `ssh user@host docker logs ric-nginx`. Usually SSL cert path/format or a typo in the generated site config.
