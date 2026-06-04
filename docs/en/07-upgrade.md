# `ric upgrade` — release a new version

`ric upgrade` ships an updated version of an already-deployed project. It preserves the database, the production credentials, the shared nginx, and the registry mirror config — only the app image and your site config are refreshed.

## Usage

```bash
# in the project directory of a project you've already deployed
ric upgrade
```

No flags, no arguments. Re-uses the same `.ric/deploy.yml` as `ric deploy`.

## What it does (in order)

1. Validates locally (same checks as deploy).
2. Builds a fresh production image (assets precompile → commit → save → sha256).
3. Opens the multiplexed SSH master.
4. Detects privilege, ensures docker, refreshes the registry mirror config if changed, verifies it.
5. **`ensureExistingDeploy`** — checks for the `current_sha` marker. If absent, errors out and tells you to run `ric deploy` first.
6. Refreshes the shared layout (no-op if already present).
7. Uploads the new image tar (skipped if the same content-addressed tar is already on the server — dedup), the SSL certs, and the freshly generated site config.
8. `docker load` and tag the new image as `<name>:<sha>`.
9. Pulls the master key from the server back to local `.ric/keys/production.key` (so a fresh clone can recover the key).
10. **Runs `db:migrate`** — only migrations; no seeds, no `db:create`. Your data is untouched.
11. **Replaces the app container** with the new image — `docker rm -f <name>` followed by `docker run` with the new image tag. The bind mounts (credentials, storage) carry over, so the new container picks up the existing database and persisted creds.
12. Reloads `ric-nginx` (`docker exec ric-nginx nginx -s reload`). The shared nginx itself is not restarted, so other projects on the server are not disrupted.
13. Writes the new `current_sha`.

Final line: `Upgraded <name> @ <short_sha> on <host>. Live at https://<domain>`.

## What's preserved

- **Database** — the bind-mounted `/var/lib/ric/<name>/storage/` carries all SQLite databases (primary, queue, cache, cable).
- **Production credentials** — `production.key` and `production.yml.enc` are generated once on first deploy and never touched again.
- **Image artifacts** — every successful upgrade's `.tar` stays under `/var/lib/ric/<name>/images/<sha>.tar`, content-addressed. Future rollback support builds on this.
- **`ric-nginx`** — the shared front door keeps running; only its config is reloaded.

## What changes

- The app container is replaced (brief downtime measured in seconds — `docker rm` then `docker run`).
- The site config is overwritten; if you changed `domain` or `ssl_dir` in `.ric/deploy.yml`, those changes take effect on this upgrade.
- The registry mirror config is rewritten if its content changed.

## When it errors

- **No existing deploy** — `current_sha` is missing, ric refuses and points you at `ric deploy`.
- **db:migrate fails** — the old container is still running, your database is untouched, and the error message tells you to fix the migration and retry. The new container is not started.

## Re-uploads are cheap

The image tar is named by its sha256, so identical content uploaded twice is detected and the second upload is skipped. If you upgrade without changing anything in the app, the network round-trip is fast.

## What's next

- A production REPL: [`ric console --remote`](04-console.md).
- Manage scheduled jobs and similar: just use Rails — they ship with the app.
