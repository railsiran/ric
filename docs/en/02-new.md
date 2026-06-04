# `ric new` — create a new project

`ric new <name>` creates a brand-new Rails project inside a Docker container, then copies the project files to your current directory so you can edit them with your favorite editor.

## Usage

```bash
ric new myapp
ric new myapp --css tailwind
ric new myapp --js three
ric new myapp --js p5 --local
```

The argument `<name>` is also the directory that will be created in your current working directory. Use lowercase letters, digits, hyphens, underscores, and dots.

## What it does (in order)

1. Picks a base image to use (see the flag table below).
2. Pulls that image from the railsiran image registry (or finds it locally, if `--local` is passed).
3. Starts a Docker container named `<name>` from that image.
4. Renames the workspace inside the container to match `<name>`.
5. Copies the project from the container to `./<name>/` on your machine.
6. Generates a fresh `config/master.key` and re-encrypts `config/credentials.yml.enc` so your project has its own unique credentials.

When it's done, you have a running container, a project directory on disk, and a host port (default `3000`, automatically incremented if `3000` is taken).

## Flags

| Flag | Image pulled | Includes |
|------|--------------|----------|
| *(no flag)* | `ri_base` | Rails 8 + SQLite + Solid Trifecta |
| `--css tailwind` | `ri_tailwind` | base + tailwindcss-rails |
| `--js three` | `ri_three` | tailwind + importmap-pinned three.js |
| `--js p5` | `ri_p5` | tailwind + p5.js |
| `--js react` | `ri_react` | tailwind + react via importmap |
| `--local` | (any of the above) | skips the download and uses an image already on your host |

`--css tailwind` and `--js …` are mutually exclusive: each `--js` variant already includes tailwind, so combining them is redundant and ric will refuse it with a helpful error.

`--local` is useful when you're building your own ric images locally (see the ric image variants docs) and want to test them without uploading to a registry.

## Common gotchas

- **The directory must not already exist.** If `./<name>/` is there, ric refuses to overwrite.
- **Name validation.** Names that start with a digit are rejected (Ruby/Rails would dislike them as module names).
- **Port collisions.** ric scans starting at `3000` and uses the first free port. If you don't see port 3000, your machine had something else listening.

## What's next

- Edit code, then push changes into the container with [`ric sync`](03-sync.md).
- Run Rails commands inside with [`ric rails`](05-rails.md).
- Open a console with [`ric console`](04-console.md).
