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

The `host` field is required. It is the IP address or hostname of your server.

The `user` field is required. It is the SSH user ric will log in as. The user must be root, or have passwordless sudo. If neither is true, ric will offer to bootstrap passwordless sudo on the first deploy by writing a `/etc/sudoers.d/ric-<user>` file — you'll be prompted for the sudo password once, then future deploys are silent.

The `ssh_key` field is optional. It is the path to the SSH private key ric should use. The `~` shortcut for your home directory is supported. If you omit this field, ric falls back to password authentication and you'll be prompted once per deploy.

The `port` field is optional. It is the SSH port; defaults to `22`. Set it explicitly if your server uses a non-standard port.

The `domain` field is required. It is the public domain your app will be served at, and it goes into the `server_name` directive of the generated nginx configuration.

The `ssl_dir` field is required. It is a **local** directory on your developer machine that contains your certificate as `fullchain.pem` and your private key as `privkey.pem`. The `~` shortcut is supported. Both files must be present; ric refuses to start the deploy if either is missing.

The `registry_mirror` field is optional. It is the host of a Docker Hub pull-through mirror that the server's Docker daemon should use. This is useful when your VPS can't reach Docker Hub directly — for example, in a country where Docker Hub is filtered or slow. ric writes this into `/etc/docker/daemon.json` on the server and verifies it works before relying on it.

## What `ric deploy` does

First, ric **validates locally** — reads `.ric/deploy.yml`, checks that your SSL files exist, and verifies the development container is running.

Second, it **builds the production image**: runs `RAILS_ENV=production bin/rails assets:precompile` inside the container, then `docker commit` plus `docker save` to produce a content-addressed `.tar` file named after its sha256. The temporary local image is cleaned up after the tar is written.

Third, it **opens an SSH master connection** that is multiplexed so you only enter your password (if any) once for the entire deploy. Subsequent ssh and scp calls reuse the same socket without re-prompting.

Fourth, it **detects your remote privilege**. If your SSH user is root, no further setup is needed. If your user has passwordless sudo, ric just prefixes commands with sudo. Otherwise, ric runs an interactive sudo command once (you'll be prompted for the sudo password) to write `/etc/sudoers.d/ric-<user>`, then continues unattended for the rest of the deploy.

Fifth, it **installs Docker** on the server if `docker` is not found. The install uses the official `get.docker.com` script.

Sixth, it **configures the registry mirror** by writing `/etc/docker/daemon.json` with your `registry_mirror` value and restarting the docker daemon — but only if the file actually needs to change. After writing, it verifies the mirror works by pulling `hello-world`.

Seventh, it **provisions the directory layout** on the server. The per-project directory is `/var/lib/ric/<project>/` and contains subdirectories for credentials, persistent storage, and image artifacts. The cross-project shared directory is `/var/lib/ric/_shared/` and contains the shared nginx sites-enabled tree, the per-project SSL trees, and houses the singleton `ric-nginx` container.

Eighth, it **uploads** the image tar (skipped if a tar with the same sha256 is already on the server), the SSL certificates, and a freshly generated nginx site config.

Ninth, it **generates production Rails credentials** on the server. ric spins up a throwaway container, runs `bin/rails credentials:edit --environment production` with `EDITOR=true` so the editor exits immediately, copies the resulting `production.key` and `production.yml.enc` to a persistent location, and destroys the throwaway container. The master key is then pulled back to your local machine and saved at `.ric/keys/production.key` with mode `0600`, so you can recover it later.

Tenth, it **runs `db:prepare`** in a one-shot container with the persistent storage volume mounted. This creates, migrates, and seeds the Solid Trifecta databases (primary, queue, cache, cable).

Eleventh, it **starts the app container** running `bin/rails server -b 0.0.0.0 -p 3000` with the environment variable `SOLID_QUEUE_IN_PUMA=true`. This means the solid_queue dispatcher and worker run in the same Puma process as web — no separate worker daemon, no foreman, no Procfile.

Twelfth, it **starts the shared `ric-nginx`** container if it isn't already running, binds host ports 80 and 443, and then reloads it so it picks up your site config. If `ric-nginx` was already running (from a previous project's deploy on this server), it's not restarted — only reloaded.

Thirteenth and last, it **writes `current_sha`** as a success marker. The presence of this file is what tells future `ric upgrade` runs that this project is deployed.

The final printed line is `Deployed <name> @ <short_sha> to <host>. Live at https://<domain>`.

## After deploy

You will see a new file at `.ric/keys/production.key` on your local machine. **Add `.ric/keys/` to your `.gitignore`** to keep the production key out of version control.

The remote layout under `/var/lib/ric/<name>/` and `/var/lib/ric/_shared/` is what subsequent `ric upgrade` runs build on.

To open a production REPL, run `ric console --remote` — see [`ric console`](04-console.md).

To push a new version of the app, run [`ric upgrade`](07-upgrade.md).

## Multiple projects on one server

`ric deploy` is designed for this. The first project to deploy starts a singleton `ric-nginx` container and binds the host's port 80 and port 443. Subsequent projects do not start their own nginx; they simply drop a site config under `/var/lib/ric/_shared/nginx/sites-enabled/<name>.conf` and `nginx -s reload` picks it up. Routing between projects is by `server_name`, which is the `domain` you configured per project.

## Re-running deploy

If you run `ric deploy` and a previous deploy already completed (the `current_sha` marker exists), ric refuses and points you at `ric upgrade`. That refusal is intentional — `deploy` is provisioning, `upgrade` is releasing.

If a previous `ric deploy` *crashed midway*, the partial state is detected on the next run and cleaned up automatically before retrying.

## Troubleshooting

If you see "the registry mirror is not serving …", your mirror value in `.ric/deploy.yml` is wrong. Most often this is a scheme like `https://` or a path included where ric only wants the hostname. ric will try to coerce common formats, but check `/etc/docker/daemon.json` on the server if it persists.

If the app container is in a restart loop, run `ssh user@host docker logs <name>`. Common culprits are a missing gem, a migration failure on first boot, or a master key decrypt issue.

If nginx is in a restart loop, run `ssh user@host docker logs ric-nginx`. The most common causes are an SSL cert path or format problem, or a syntax error in the generated site config.
