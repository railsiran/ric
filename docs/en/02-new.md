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

First, it picks a base image to use based on the flags you passed (described below).

Second, it pulls that image from the railsiran image registry. If you passed `--local`, it looks for the image already on your machine instead and skips the download.

Third, it starts a Docker container named `<name>` from that image.

Fourth, it renames the workspace inside the container to match `<name>`.

Fifth, it copies the project from the container to `./<name>/` on your machine so your editor can see the files.

Sixth, it generates a fresh `config/master.key` and re-encrypts `config/credentials.yml.enc` so your project has its own unique credentials. Two projects created with `ric new` never share a master key.

When all six steps are done, you have a running container, a project directory on disk, and a host port (default `3000`, automatically incremented if `3000` is taken).

## The flags

### No flag

```bash
ric new myapp
```

Pulls the `ri_base` image. This is the most minimal variant: Rails 8 with SQLite and the Solid Trifecta (solid_queue, solid_cache, solid_cable). No CSS framework, no JavaScript library. Use this when you want to start from a clean slate or when you'll add your own asset stack.

### `--css tailwind`

```bash
ric new myapp --css tailwind
```

Pulls the `ri_tailwind` image. Same as the base, plus `tailwindcss-rails` is already installed and configured. You can write Tailwind utility classes in your views from the first commit.

### `--js three`

```bash
ric new myapp --js three
```

Pulls the `ri_three` image. This is the Tailwind image plus three.js pinned in importmap. Use this when you want WebGL or 3D graphics ready to go — no separate `npm install` step needed.

Because the `three` variant is built on top of the Tailwind variant, Tailwind is already included. You do not need to pass `--css tailwind` as well; ric will reject that combination as redundant.

### `--js p5`

```bash
ric new myapp --js p5
```

Pulls the `ri_p5` image. Tailwind plus p5.js, again via importmap. Use this for creative-coding sketches, generative art, or quick visualizations.

Same rule as `--js three`: Tailwind is already inside, so don't combine with `--css tailwind`.

### `--js react`

```bash
ric new myapp --js react
```

Pulls the `ri_react` image. Tailwind plus React, set up to load via importmap. Use this when you want to write React components inside a Rails app without going to a separate Node toolchain.

Same rule: Tailwind is already inside, so don't combine with `--css tailwind`.

### `--local`

```bash
ric new myapp --local
ric new myapp --js three --local
```

Skips the download step and uses an image that already exists on your local Docker. This is useful when you are building your own ric images locally (see the ric image variants documentation) and want to test them without uploading to a registry first. It can be combined with any of the variant flags.

## Common gotchas

The directory must not already exist. If `./<name>/` is there, ric refuses to overwrite — rename or remove it first.

Names that start with a digit are rejected, because Ruby and Rails would dislike them as module names. The full set of allowed characters is lowercase letters, digits, hyphens, underscores, and dots.

Port collisions are handled automatically: ric scans starting at `3000` and uses the first free port. If you don't see your project on port 3000, your machine had something else listening there.

## What's next

After editing code, push changes into the container with [`ric sync`](03-sync.md).

To run Rails commands inside the container, see [`ric rails`](05-rails.md).

To open an interactive console, see [`ric console`](04-console.md).
