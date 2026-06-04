# Quick install

`ric` is a single self-contained binary written in Go. There is no installer, no package to register, no dependencies to compile.

## Prerequisites

You need three things on the machine where you'll run `ric`:

1. **Docker** — `ric` does all of its work inside Docker containers. Install Docker Desktop on macOS/Windows or `docker` + `docker compose` on Linux. Confirm with `docker --version`.
2. **An SSH client** — only required if you plan to use `ric deploy` and `ric upgrade`. macOS, Linux, and modern Windows all ship one.
3. **Internet access** — first-time `ric new` downloads a Docker image from the railsiran image registry. Subsequent runs reuse the local copy.

You do **not** need a Ruby installation on the host, you do **not** need bundler, and you do **not** need any Rails gems. Everything Ruby-related lives inside the container.

## Get the binary

Download the build for your platform from the latest release on the railsiran website. The names tell you which file to grab:

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `ric-darwin-arm64` |
| Linux x86_64 | `ric-linux-amd64` |
| Linux ARM (Raspberry Pi, AWS Graviton…) | `ric-linux-arm64` |
| Windows | `ric-windows-amd64.exe` |

## Make it executable and put it on your PATH

On macOS/Linux:

```bash
chmod +x ~/Downloads/ric-darwin-arm64
sudo mv ~/Downloads/ric-darwin-arm64 /usr/local/bin/ric
```

On Windows, drop `ric-windows-amd64.exe` into a folder that's on your `PATH` (or add the folder to PATH via System Properties → Environment Variables).

## Verify

```bash
ric version
```

You should see the version number and codename of the build. If you get "command not found", the binary isn't on your PATH yet.

## What's next

- New to ric? Run `ric new myapp` — see [`ric new`](02-new.md).
- Ready to deploy? See [`ric deploy`](06-deploy.md).
