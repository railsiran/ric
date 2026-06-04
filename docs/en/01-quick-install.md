# Quick install

`ric` is a single self-contained binary written in Go. There is no installer, no package to register, no dependencies to compile.

## Prerequisites

You need three things on the machine where you'll run `ric`.

The first is **Docker**. `ric` does all of its work inside Docker containers. Install Docker Desktop on macOS or Windows, or `docker` plus `docker compose` on Linux. Confirm the installation with:

```bash
docker --version
```

The second is **an SSH client**, but only if you plan to use `ric deploy` and `ric upgrade`. macOS, Linux, and modern Windows all ship one — no extra installation needed.

The third is **internet access** on first use. The first time you run `ric new`, it downloads a Docker image from the railsiran image registry. Subsequent runs reuse the local copy and work offline.

You do **not** need a Ruby installation on the host, you do **not** need bundler, and you do **not** need any Rails gems. Everything Ruby-related lives inside the container.

## Get the binary

Download the build for your platform from the latest release on the railsiran website. There are four builds available, one for each common platform.

On **macOS with Apple Silicon** (M1, M2, M3, M4) the file you want is named `ric-darwin-arm64`.

On **Linux x86_64** — most desktop PCs, most VPS providers, most cloud VMs — the file is named `ric-linux-amd64`.

On **Linux ARM** — Raspberry Pi, AWS Graviton, Oracle Ampere, and similar ARM-based servers — the file is named `ric-linux-arm64`.

On **Windows** the file is named `ric-windows-amd64.exe`.

## Make it executable and put it on your PATH

On macOS or Linux, mark the file as executable and move it into a directory that is already on your PATH. The standard choice is `/usr/local/bin`. For example, if you downloaded the macOS Apple Silicon build:

```bash
chmod +x ~/Downloads/ric-darwin-arm64
sudo mv ~/Downloads/ric-darwin-arm64 /usr/local/bin/ric
```

After this move, the file is simply called `ric`, with no platform suffix.

On Windows, drop `ric-windows-amd64.exe` into a folder that is already on your `PATH`. If you don't have one set up, create something like `C:\Tools\` and add it via System Properties → Environment Variables → Path. You may also want to rename the file to `ric.exe` for convenience so you can run `ric` instead of `ric-windows-amd64`.

## Verify

```bash
ric version
```

You should see the version number and codename of the build. If you get "command not found", the binary isn't on your PATH yet.

## What's next

If you're new to ric, run `ric new myapp` — see [`ric new`](02-new.md).

If you're ready to deploy an existing project, see [`ric deploy`](06-deploy.md).
