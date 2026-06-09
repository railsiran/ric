package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ric/dispatcher"
)

type DeployConfig struct {
	Host           string
	User           string
	SSHKey         string
	Port           int
	Domain         string
	SSLDir         string
	RegistryMirror string
	Memory         string // optional: docker --memory for the app container (e.g. "512m")
	CPUs           string // optional: docker --cpus for the app container (e.g. "1.5")
}

type deploySession struct {
	cfg         *DeployConfig
	name        string
	controlPath string
	sudoPrefix  string // "" if remote user is root; "sudo" otherwise
}

func (s *deploySession) root() string { return "/var/lib/ric/" + s.name }
func (s *deploySession) target() string {
	return s.cfg.User + "@" + s.cfg.Host
}

// --- entry point ---

func Deploy(inputs []string, flagArgs []string) error {
	if len(flagArgs) > 0 {
		return fmt.Errorf("ric deploy takes no flags (got %s)", flagArgs[0])
	}
	if len(inputs) > 0 {
		return fmt.Errorf("ric deploy takes no positional arguments (got %q)", inputs[0])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	cfg, err := loadDeployConfig()
	if err != nil {
		return err
	}
	if err := validateLocalArtifacts(cfg); err != nil {
		return err
	}
	if err := validateProductionDB(); err != nil {
		return err
	}
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	step("Precompiling assets for production")
	if err := dockerExec(name, "/workspace/"+name,
		[]string{"RAILS_ENV=production"},
		"bin/rails", "assets:precompile"); err != nil {
		return fmt.Errorf("assets:precompile failed in dev container %s: %w", name, err)
	}
	done("Assets precompiled")

	step("Building deployment image")
	tarPath, sha, err := buildImage(name)
	if err != nil {
		return err
	}
	defer os.Remove(tarPath)
	done(fmt.Sprintf("Built %s @ %s", filepath.Base(tarPath), shortSHA(sha)))

	sess := &deploySession{
		cfg:         cfg,
		name:        name,
		controlPath: fmt.Sprintf("/tmp/ric-cm-%s-%d", name, os.Getpid()),
	}

	step(fmt.Sprintf("Opening SSH connection to %s@%s", cfg.User, cfg.Host))
	if err := sess.openMaster(); err != nil {
		return err
	}
	defer sess.closeMaster()
	done("SSH connection ready")

	for _, step := range []func() error{
		sess.detectPrivilege,
		sess.ensureDocker,
		sess.configureDockerMirror,
		sess.verifyRegistryMirror,
		sess.ensureFirstDeploy,
		sess.provisionSharedLayout,
		sess.provisionLayout,
		func() error { return sess.uploadArtifacts(tarPath, sha) },
		func() error { return sess.loadAndTagImage(sha) },
		sess.ensureNetwork,
		func() error { return sess.generateProdCredentials(sha) },
		sess.syncMasterKeyLocal,
		func() error { return sess.runDBPrepare(sha) },
		func() error { return sess.startApp(sha) },
		sess.ensureSharedNginx,
		sess.reloadNginx,
		func() error { return sess.writeCurrentSHA(sha) },
	} {
		if err := step(); err != nil {
			return err
		}
	}

	fmt.Printf("\nDeployed %s @ %s to %s. Live at https://%s\n",
		name, shortSHA(sha), cfg.Host, cfg.Domain)
	return nil
}

// --- config + local prerequisites ---

func loadDeployConfig() (*DeployConfig, error) {
	const path = ".ric/deploy.yml"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	cfg := &DeployConfig{Port: 22}
	for n, line := range strings.Split(string(data), "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			return nil, fmt.Errorf("%s line %d: expected 'key: value', got %q", path, n+1, raw)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "host":
			cfg.Host = value
		case "user":
			cfg.User = value
		case "ssh_key":
			cfg.SSHKey = value
		case "port":
			p, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid port %q", path, value)
			}
			cfg.Port = p
		case "domain":
			cfg.Domain = value
		case "ssl_dir":
			cfg.SSLDir = value
		case "registry_mirror":
			cfg.RegistryMirror = value
		case "memory":
			cfg.Memory = value
		case "cpus":
			cfg.CPUs = value
		default:
			return nil, fmt.Errorf("%s line %d: unknown key %q", path, n+1, key)
		}
	}

	cfg.SSHKey, err = expandHome(cfg.SSHKey)
	if err != nil {
		return nil, err
	}
	cfg.SSLDir, err = expandHome(cfg.SSLDir)
	if err != nil {
		return nil, err
	}

	for field, val := range map[string]string{
		"host":    cfg.Host,
		"user":    cfg.User,
		"domain":  cfg.Domain,
		"ssl_dir": cfg.SSLDir,
	} {
		if val == "" {
			return nil, fmt.Errorf("%s: %q is required", path, field)
		}
	}
	return cfg, nil
}

func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand ~ in %q: %w", p, err)
	}
	return filepath.Join(home, p[2:]), nil
}

func validateLocalArtifacts(cfg *DeployConfig) error {
	if cfg.SSHKey != "" {
		if _, err := os.Stat(cfg.SSHKey); err != nil {
			return fmt.Errorf("ssh_key not found at %s — fix 'ssh_key' in .ric/deploy.yml or remove the field to use password auth", cfg.SSHKey)
		}
	}
	for _, f := range []string{"fullchain.pem", "privkey.pem"} {
		p := filepath.Join(cfg.SSLDir, f)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("ssl_dir is missing %s at %s", f, p)
		}
	}
	return nil
}

// validateProductionDB catches the most common deploy-time footgun for
// projects migrated onto a ric image: a config/database.yml whose production
// block still has its `database:` paths commented out (the stock Rails
// template ships them as `# database: path/to/...` placeholders). Left
// uncommented, db:prepare dies on the server with the cryptic
// "No database file specified. Missing argument: database".
//
// We fail fast, locally, with an actionable message instead of letting it
// blow up mid-deploy on the remote.
func validateProductionDB() error {
	const path = "config/database.yml"
	data, err := os.ReadFile(path)
	if err != nil {
		// No database.yml is unusual for a ric app, but not our call to make —
		// leave it to Rails. Only validate when the file is present.
		return nil
	}

	lines := strings.Split(string(data), "\n")
	inProd := false
	var haveDatabase, placeholders int
	for _, line := range lines {
		// Top-level key (no indentation) ends the production block.
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			inProd = strings.HasPrefix(line, "production:")
			continue
		}
		if !inProd {
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "database:"):
			haveDatabase++
		case strings.HasPrefix(trimmed, "# database:"),
			strings.HasPrefix(trimmed, "#database:"):
			placeholders++
		}
	}

	if haveDatabase == 0 && placeholders > 0 {
		return fmt.Errorf("%s: the production block has commented-out 'database:' paths.\n"+
			"This is the stock Rails placeholder and will make db:prepare fail on the server with "+
			"\"No database file specified\".\n"+
			"Uncomment and set the paths, e.g.:\n"+
			"  production:\n"+
			"    primary:\n"+
			"      <<: *default\n"+
			"      database: storage/production.sqlite3\n"+
			"    cache:\n"+
			"      <<: *default\n"+
			"      database: storage/production_cache.sqlite3\n"+
			"      migrations_paths: db/cache_migrate\n"+
			"    queue:\n"+
			"      <<: *default\n"+
			"      database: storage/production_queue.sqlite3\n"+
			"      migrations_paths: db/queue_migrate\n"+
			"    cable:\n"+
			"      <<: *default\n"+
			"      database: storage/production_cable.sqlite3\n"+
			"      migrations_paths: db/cable_migrate\n"+
			"Then run 'ric sync' and deploy again.", path)
	}
	return nil
}

// ensureSQLitePragmas patches the dev container's config/database.yml to add
// production SQLite pragmas (WAL journal mode etc.) under the `default:`
// anchor, where all environments and all Solid Trifecta databases inherit
// them. It reads the file out of the container, transforms it in memory, and
// writes it back — the developer's host file is left untouched.
//
// Idempotent: if pragmas are already present, or the adapter isn't sqlite3,
// or the file has no `default:` anchor, it does nothing.
func ensureSQLitePragmas(name string) error {
	const rel = "config/database.yml"
	containerPath := "/workspace/" + name + "/" + rel

	out, err := exec.Command("docker", "exec", "-w", "/workspace/"+name, name,
		"cat", rel).Output()
	if err != nil {
		// No database.yml in the container is unusual but not fatal here — the
		// earlier validateProductionDB ran against the host copy. Skip quietly.
		return nil
	}

	patched, changed := addSQLitePragmas(string(out))
	if !changed {
		return nil
	}

	step("Applying SQLite production pragmas (WAL mode)")
	tmp, err := os.CreateTemp("", name+"-database-*.yml")
	if err != nil {
		return fmt.Errorf("create temp database.yml: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(patched); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp database.yml: %w", err)
	}
	tmp.Close()

	cp := exec.Command("docker", "cp", tmp.Name(), name+":"+containerPath)
	cp.Stderr = os.Stderr
	if err := cp.Run(); err != nil {
		return fmt.Errorf("copy patched database.yml into %s: %w", name, err)
	}
	done("SQLite pragmas applied to the deploy image")
	return nil
}

// addSQLitePragmas inserts a `pragmas:` block under the `default: &anchor`
// mapping of a Rails database.yml. Returns the new content and whether a
// change was made. Pure function — no I/O — so it can be unit tested.
//
// It is conservative: it only acts when the file uses the sqlite3 adapter, has
// a `default:` anchor, and doesn't already declare `pragmas:`.
func addSQLitePragmas(yaml string) (string, bool) {
	if strings.Contains(yaml, "pragmas:") {
		return yaml, false
	}
	if !strings.Contains(yaml, "adapter: sqlite3") {
		return yaml, false
	}

	lines := strings.Split(yaml, "\n")
	anchorIdx := -1
	for i, line := range lines {
		// Match a top-level `default:` carrying a YAML anchor (`&default`).
		if strings.HasPrefix(line, "default:") && strings.Contains(line, "&") {
			anchorIdx = i
			break
		}
	}
	if anchorIdx == -1 {
		return yaml, false
	}

	// Determine the indentation used for keys inside the default block by
	// looking at the first indented, non-empty line after the anchor.
	childIndent := "  "
	for _, line := range lines[anchorIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		ws := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if ws == "" {
			break // hit the next top-level key without finding a child
		}
		childIndent = ws
		break
	}
	inner := childIndent + childIndent

	block := []string{
		childIndent + "pragmas:",
		inner + "journal_mode: WAL",
		inner + "synchronous: NORMAL",
		inner + "busy_timeout: 5000",
		inner + "foreign_keys: true",
		inner + "cache_size: -64000", // ~64 MB page cache
		inner + "mmap_size: 134217728", // 128 MB memory-mapped I/O
	}

	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:anchorIdx+1]...)
	out = append(out, block...)
	out = append(out, lines[anchorIdx+1:]...)
	return strings.Join(out, "\n"), true
}

// --- local docker helpers ---

// dockerExec runs `docker exec -w <wd> -e <env...> <container> <cmd...>` and
// streams output to the caller's terminal.
func dockerExec(container, workdir string, env []string, cmd ...string) error {
	args := []string{"exec", "-w", workdir}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, container)
	args = append(args, cmd...)
	c := exec.Command("docker", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func buildImage(name string) (tarPath, sha string, err error) {
	// Apply production-grade SQLite pragmas to the container's database.yml
	// before committing, so the deployed image runs in WAL mode etc. This is
	// the highest-impact scaling fix for the SQLite + Solid Trifecta stack:
	// without WAL, a writer blocks all readers and the constant Solid Trifecta
	// writes cause SQLITE_BUSY stalls under load.
	if err := ensureSQLitePragmas(name); err != nil {
		return "", "", err
	}

	tag := name + ":ric-deploy"
	commit := exec.Command("docker", "commit", name, tag)
	commit.Stdout, commit.Stderr = os.Stdout, os.Stderr
	if err := commit.Run(); err != nil {
		return "", "", fmt.Errorf("docker commit %s failed: %w", name, err)
	}

	tarPath = filepath.Join(os.TempDir(), name+".tar")
	save := exec.Command("docker", "save", "-o", tarPath, tag)
	save.Stdout, save.Stderr = os.Stdout, os.Stderr
	if err := save.Run(); err != nil {
		return "", "", fmt.Errorf("docker save %s failed: %w", tag, err)
	}

	// The committed image is captured in the tar; we don't need it locally.
	// Removing it frees the duplicated layers on the host.
	if err := exec.Command("docker", "rmi", tag).Run(); err != nil {
		fmt.Printf("  (note) could not remove temporary image %s: %v\n", tag, err)
	}

	sha, err = sha256File(tarPath)
	if err != nil {
		return "", "", err
	}
	return tarPath, sha, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s for hashing: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func shortSHA(sha string) string {
	if len(sha) < 12 {
		return sha
	}
	return sha[:12]
}

// --- SSH session ---

// connArgs returns the common ssh/scp option flags. portFlag is "-p" for ssh
// or "-P" for scp; both tools share everything else.
func (s *deploySession) connArgs(portFlag string) []string {
	args := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + s.controlPath,
		"-o", "ControlPersist=10m",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ServerAliveInterval=30",
	}
	if s.cfg.SSHKey != "" {
		args = append(args, "-i", s.cfg.SSHKey)
	}
	if s.cfg.Port != 0 && s.cfg.Port != 22 {
		args = append(args, portFlag, strconv.Itoa(s.cfg.Port))
	}
	return args
}

func (s *deploySession) sshArgs() []string { return s.connArgs("-p") }
func (s *deploySession) scpArgs() []string { return s.connArgs("-P") }

// openMaster establishes the multiplexed SSH master in the background. After
// this, the user enters their password (if any) at most once for the entire
// deploy — every subsequent ssh / scp call reuses the socket silently.
func (s *deploySession) openMaster() error {
	args := append(s.sshArgs(), "-fN", "-M", s.target())
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		mode := "(password auth)"
		if s.cfg.SSHKey != "" {
			mode = "(key: " + s.cfg.SSHKey + ")"
		}
		return fmt.Errorf("ssh connection to %s failed %s: %w", s.target(), mode, err)
	}
	return nil
}

func (s *deploySession) closeMaster() {
	exec.Command("ssh", append(s.sshArgs(), "-O", "exit", s.target())...).Run()
	os.Remove(s.controlPath)
}

func (s *deploySession) ssh(action, cmd string) error {
	c := exec.Command("ssh", append(s.sshArgs(), s.target(), cmd)...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s on %s failed: %w", action, s.cfg.Host, err)
	}
	return nil
}

// sshQuiet runs a command, returning its trimmed stdout and whether the exit
// status was zero. Errors are intentionally suppressed; callers use the bool
// result to make a decision.
func (s *deploySession) sshQuiet(cmd string) (stdout string, ok bool) {
	out, err := exec.Command("ssh", append(s.sshArgs(), s.target(), cmd)...).Output()
	return strings.TrimSpace(string(out)), err == nil
}

// sshInteractive wires stdin and a pty through the master connection. Used
// only for the one-time NOPASSWD-sudo bootstrap where the remote sudo needs
// to prompt for a password.
func (s *deploySession) sshInteractive(action, cmd string) error {
	c := exec.Command("ssh", append(s.sshArgs(), "-t", s.target(), cmd)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s on %s failed: %w", action, s.cfg.Host, err)
	}
	return nil
}

func (s *deploySession) scp(action, local, remote string) error {
	c := exec.Command("scp", append(s.scpArgs(), local, s.target()+":"+remote)...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s (scp %s → %s) failed: %w", action, local, remote, err)
	}
	return nil
}

// --- privilege bootstrap ---

// detectPrivilege figures out how privileged commands should be invoked.
// If the SSH user is neither root nor a passwordless sudoer, ric attempts a
// one-time bootstrap: prompt for the sudo password and write
// /etc/sudoers.d/ric-<user> so future deploys (and the rest of this one) are
// unattended.
func (s *deploySession) detectPrivilege() error {
	uid, ok := s.sshQuiet("id -u")
	if !ok {
		return fmt.Errorf("could not query remote user id on %s", s.cfg.Host)
	}
	if uid == "0" {
		return nil
	}
	if _, ok := s.sshQuiet("sudo -n true 2>/dev/null && echo ok"); ok {
		s.sudoPrefix = "sudo"
		fmt.Printf("  Remote user %q is not root; using passwordless sudo.\n", s.cfg.User)
		return nil
	}

	fmt.Printf("\n  Remote user %q has neither root nor passwordless sudo.\n", s.cfg.User)
	fmt.Printf("  ric will configure /etc/sudoers.d/ric-%s to grant passwordless sudo.\n", s.cfg.User)
	fmt.Printf("  You'll be prompted for the sudo password on %s once.\n\n", s.cfg.Host)

	bootstrap := fmt.Sprintf(
		`sudo sh -c 'echo "%s ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/ric-%s && chmod 0440 /etc/sudoers.d/ric-%s && visudo -cf /etc/sudoers.d/ric-%s'`,
		s.cfg.User, s.cfg.User, s.cfg.User, s.cfg.User)
	if err := s.sshInteractive("configuring sudoers", bootstrap); err != nil {
		return fmt.Errorf("could not configure passwordless sudo for %q on %s. "+
			"Either grant passwordless sudo manually (add '%s ALL=(ALL) NOPASSWD:ALL' to /etc/sudoers.d/ric-%s) "+
			"or set 'user: root' in .ric/deploy.yml. Underlying: %w",
			s.cfg.User, s.cfg.Host, s.cfg.User, s.cfg.User, err)
	}
	if _, ok := s.sshQuiet("sudo -n true 2>/dev/null && echo ok"); !ok {
		return fmt.Errorf("configured /etc/sudoers.d/ric-%s but 'sudo -n true' still fails on %s — review the file manually", s.cfg.User, s.cfg.Host)
	}
	s.sudoPrefix = "sudo"
	fmt.Printf("  ✓ Passwordless sudo configured for %q. Future deploys won't need this step.\n", s.cfg.User)
	return nil
}

// priv wraps a shell command so it runs with the appropriate privilege.
// Wraps in `sudo sh -c '<cmd>'` (not just prepending `sudo`) so the privilege
// elevation applies to the whole pipeline including redirects and pipes, not
// only the first word.
func (s *deploySession) priv(cmd string) string {
	if s.sudoPrefix == "" {
		return cmd
	}
	return s.sudoPrefix + " sh -c " + shellQuote(cmd)
}

// --- remote provisioning ---

func (s *deploySession) ensureDocker() error {
	if _, ok := s.sshQuiet("command -v docker"); ok {
		return nil
	}
	step(fmt.Sprintf("Installing Docker on %s (this may take 1–2 minutes)", s.cfg.Host))
	if err := s.ssh("docker install", s.priv("curl -fsSL https://get.docker.com | sh")); err != nil {
		return fmt.Errorf("docker install failed on %s: %w", s.cfg.Host, err)
	}
	if _, ok := s.sshQuiet("command -v docker"); !ok {
		return fmt.Errorf("docker still not found on %s after install attempt", s.cfg.Host)
	}
	done("Docker installed")
	return nil
}

// configureDockerMirror writes /etc/docker/daemon.json with the configured
// registry_mirror as a Docker Hub pull-through. The file is uploaded to /tmp
// first and moved into place only when its content changes — avoids restarting
// docker (and disrupting other projects' containers) on no-op reruns.
func (s *deploySession) configureDockerMirror() error {
	if s.cfg.RegistryMirror == "" {
		return nil
	}
	mirror := registryMirrorURL(s.cfg.RegistryMirror)
	desired := fmt.Sprintf("{\"registry-mirrors\":[\"%s\"]}\n", mirror)

	step(fmt.Sprintf("Configuring Docker registry mirror %s", mirror))

	localPath := filepath.Join(os.TempDir(), s.name+"-daemon.json")
	if err := os.WriteFile(localPath, []byte(desired), 0644); err != nil {
		return fmt.Errorf("write local %s: %w", localPath, err)
	}
	defer os.Remove(localPath)

	remoteStage := "/tmp/ric-daemon.json"
	if err := s.scp("upload daemon.json", localPath, remoteStage); err != nil {
		return err
	}

	apply := fmt.Sprintf(
		"mkdir -p /etc/docker && "+
			"if ! cmp -s %[1]s /etc/docker/daemon.json 2>/dev/null; then "+
			"  mv %[1]s /etc/docker/daemon.json && "+
			"  chmod 0644 /etc/docker/daemon.json && "+
			"  systemctl restart docker; "+
			"else "+
			"  rm -f %[1]s; "+
			"fi",
		remoteStage)
	if err := s.ssh("apply daemon.json", s.priv(apply)); err != nil {
		return fmt.Errorf("could not apply /etc/docker/daemon.json on %s: %w", s.cfg.Host, err)
	}

	out, ok := s.sshQuiet("docker info --format '{{range .RegistryConfig.Mirrors}}{{.}}{{end}}'")
	if !ok || !strings.Contains(out, mirror) {
		return fmt.Errorf("daemon.json was written but docker doesn't report %s in 'docker info' (got %q). "+
			"Check /etc/docker/daemon.json on %s and 'journalctl -u docker --since 1min'.",
			mirror, out, s.cfg.Host)
	}
	done("Registry mirror configured (" + strings.TrimSpace(out) + ")")
	return nil
}

// verifyRegistryMirror probes the configured mirror by actually pulling a
// tiny image (hello-world). This goes through the daemon's pull path, which
// is the only path that respects registry-mirrors in daemon.json.
//
// Notably, `docker manifest inspect` does NOT use the mirror — it's a
// client-side call straight to the registry — so a successful manifest
// inspect would prove the wrong thing.
//
// hello-world is ~2 KiB so the warm-up cost is negligible and the image is
// removed right after; the same code path is what nginx pull will exercise
// next, so success here is a meaningful precondition.
func (s *deploySession) verifyRegistryMirror() error {
	if s.cfg.RegistryMirror == "" {
		return nil
	}
	step("Verifying registry mirror via hello-world pull")
	probe := "docker pull hello-world:latest && docker rmi hello-world:latest >/dev/null 2>&1; true"
	if err := s.ssh("docker pull hello-world:latest", s.priv(probe)); err != nil {
		return fmt.Errorf("the registry mirror %q on %s is not serving hello-world:latest. "+
			"Check /etc/docker/daemon.json on the remote and that the mirror is a Docker Hub pull-through. Underlying: %w",
			s.cfg.RegistryMirror, s.cfg.Host, err)
	}
	done("Registry mirror is reachable")
	return nil
}

// ensureFirstDeploy decides whether this is a true first deploy. The success
// marker is /var/lib/ric/<name>/current_sha, written as the very last step of
// a completed deploy. Three cases:
//   - current_sha exists → refuse; point at upgrade.
//   - dir exists, no current_sha → partial state from a crashed run; clean up.
//   - dir doesn't exist → fresh deploy.
func (s *deploySession) ensureFirstDeploy() error {
	root := s.root()
	if _, ok := s.sshQuiet("test -f " + shellQuote(root+"/current_sha") + " && echo yes"); ok {
		return fmt.Errorf("project %q is already deployed on %s (found %s/current_sha). "+
			"Use 'ric upgrade' for subsequent releases — it preserves the database and production credentials.",
			s.name, s.cfg.Host, root)
	}
	if _, ok := s.sshQuiet("test -d " + shellQuote(root) + " && echo yes"); ok {
		fmt.Printf("  Found partial state from a previous failed deploy at %s — cleaning up.\n", root)
		cleanup := fmt.Sprintf(
			"rm -rf %s %s/ssl/%s %s/nginx/sites-enabled/%s.conf; "+
				"docker rm -f %s >/dev/null 2>&1; true",
			shellQuote(root),
			shellQuote(sharedRoot), s.name,
			shellQuote(sharedRoot), s.name,
			s.name)
		if err := s.ssh("clean up partial deploy", s.priv(cleanup)); err != nil {
			return fmt.Errorf("could not clean up partial state at %s — remove it manually and retry: %w", root, err)
		}
	}
	return nil
}

// sharedRoot is where the cross-project nginx, ssl, and sites-enabled live.
// One ric-nginx container fronts every project's app container by hostname.
const sharedRoot = "/var/lib/ric/_shared"

// provisionSharedLayout creates the host-wide directories used by every
// project: shared nginx sites-enabled and per-project SSL trees. Idempotent;
// runs on every deploy and upgrade.
func (s *deploySession) provisionSharedLayout() error {
	dirs := []string{
		sharedRoot,
		sharedRoot + "/nginx/sites-enabled",
		sharedRoot + "/ssl",
		sharedRoot + "/ssl/" + s.name,
	}
	quoted := make([]string, len(dirs))
	for i, d := range dirs {
		quoted[i] = shellQuote(d)
	}
	cmd := fmt.Sprintf("mkdir -p %s && chown -R %s %s",
		strings.Join(quoted, " "), s.cfg.User, shellQuote(sharedRoot))
	return s.ssh("provisioning "+sharedRoot, s.priv(cmd))
}

func (s *deploySession) provisionLayout() error {
	// Enumerate subdirs explicitly. Brace expansion is bash-only — would silently
	// become a literal directory name under dash (/bin/sh on Debian/Ubuntu) and
	// break the later scp into images/.
	subs := []string{"credentials", "storage", "images"}
	paths := make([]string, len(subs))
	for i, sub := range subs {
		paths[i] = shellQuote(s.root() + "/" + sub)
	}
	// chown so subsequent scp calls write directly without sudo. Container-
	// written files (root inside docker) stay root-owned on the host — that's
	// fine; ric only needs the directory itself to be user-writable.
	cmd := fmt.Sprintf("mkdir -p %s && chown -R %s %s",
		strings.Join(paths, " "), s.cfg.User, shellQuote(s.root()))
	return s.ssh("provisioning "+s.root(), s.priv(cmd))
}

func (s *deploySession) uploadArtifacts(tarPath, sha string) error {
	root := s.root()
	remoteTar := fmt.Sprintf("%s/images/%s.tar", root, sha)

	if _, ok := s.sshQuiet(fmt.Sprintf("test -f %s && echo yes", shellQuote(remoteTar))); ok {
		fmt.Printf("  Image %s already on host, skipping upload\n", shortSHA(sha))
	} else {
		step(fmt.Sprintf("Uploading image (%s) to %s", shortSHA(sha), s.cfg.Host))
		if err := s.scp("upload image", tarPath, remoteTar); err != nil {
			return err
		}
		done("Image uploaded")
	}

	step("Uploading SSL certificates")
	for _, f := range []string{"fullchain.pem", "privkey.pem"} {
		dst := sharedRoot + "/ssl/" + s.name + "/" + f
		if err := s.scp("upload "+f, filepath.Join(s.cfg.SSLDir, f), dst); err != nil {
			return err
		}
	}
	done("SSL certificates uploaded to " + sharedRoot + "/ssl/" + s.name)

	step("Uploading generated nginx site config")
	confLocal := filepath.Join(os.TempDir(), s.name+"-nginx.conf")
	if err := os.WriteFile(confLocal, []byte(renderNginxConf(s.name, s.cfg.Domain)), 0644); err != nil {
		return fmt.Errorf("write nginx config to %s: %w", confLocal, err)
	}
	defer os.Remove(confLocal)
	dst := sharedRoot + "/nginx/sites-enabled/" + s.name + ".conf"
	if err := s.scp("upload site config", confLocal, dst); err != nil {
		return err
	}
	done("Nginx site config uploaded")
	return nil
}

func (s *deploySession) loadAndTagImage(sha string) error {
	step("Loading image into Docker on remote")
	tar := fmt.Sprintf("%s/images/%s.tar", s.root(), sha)
	if err := s.ssh("docker load", s.priv("docker load -i "+shellQuote(tar))); err != nil {
		return err
	}
	if err := s.ssh("docker tag",
		s.priv(fmt.Sprintf("docker tag %s:ric-deploy %s:%s", s.name, s.name, sha))); err != nil {
		return err
	}
	done("Image loaded")
	return nil
}

func (s *deploySession) ensureNetwork() error {
	return s.ssh("create docker network",
		s.priv("docker network inspect ric-net >/dev/null 2>&1 || docker network create ric-net"))
}

// Sane Puma defaults for a small single-VPS deploy: 2 worker processes, each
// with a 3-thread pool → up to 6 concurrent requests. RAILS_MAX_THREADS also
// sizes the ActiveRecord connection pool. Both are passed as env vars so a
// project's config/puma.rb ENV.fetch lookups pick them up; a project can still
// override via its own config if it reads different vars.
const (
	defaultWebConcurrency = "2"
	defaultMaxThreads     = "3"
)

// appMounts returns the bind-mount + workdir + env args shared by every
// container ric launches for the app (credgen, db:prepare, the live app).
// SOLID_QUEUE_IN_PUMA enables Rails 8's puma plugin for solid_queue so the
// queue dispatcher + worker run in-process with web — no separate daemon
// needed. (solid_cache and solid_cable are in-process by their nature.)
func (s *deploySession) appMounts() string {
	return fmt.Sprintf(
		"-v %[1]s/credentials:/workspace/%[2]s/config/credentials "+
			"-v %[1]s/storage:/workspace/%[2]s/storage "+
			"-w /workspace/%[2]s "+
			"-e RAILS_ENV=production -e SOLID_QUEUE_IN_PUMA=true "+
			"-e WEB_CONCURRENCY=%[3]s -e RAILS_MAX_THREADS=%[4]s",
		s.root(), s.name, defaultWebConcurrency, defaultMaxThreads)
}

// resourceFlags returns docker --memory / --cpus flags when the user has set
// them in .ric/deploy.yml. Empty (unconstrained) by default.
func (s *deploySession) resourceFlags() string {
	var parts []string
	if s.cfg.Memory != "" {
		parts = append(parts, "--memory "+s.cfg.Memory)
	}
	if s.cfg.CPUs != "" {
		parts = append(parts, "--cpus "+s.cfg.CPUs)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

func (s *deploySession) generateProdCredentials(sha string) error {
	keyPath := s.root() + "/credentials/production.key"
	if _, ok := s.sshQuiet("test -f " + shellQuote(keyPath) + " && echo yes"); ok {
		// Should not happen on a true first deploy (the layout-existence guard
		// would have caught it), but be defensive against partial runs.
		return nil
	}

	step("Generating production Rails credentials on remote")
	credgen := s.name + "-credgen-" + shortSHA(sha)
	imageRef := fmt.Sprintf("%s:%s", s.name, sha)
	credsDir := s.root() + "/credentials"

	cmd := strings.Join([]string{
		fmt.Sprintf("docker rm -f %s >/dev/null 2>&1; true", credgen),
		fmt.Sprintf("docker run --name %s -w /workspace/%s -e EDITOR=true %s "+
			"bin/rails credentials:edit --environment production",
			credgen, s.name, imageRef),
		fmt.Sprintf("docker cp %s:/workspace/%s/config/credentials/production.key %s/",
			credgen, s.name, credsDir),
		fmt.Sprintf("docker cp %s:/workspace/%s/config/credentials/production.yml.enc %s/",
			credgen, s.name, credsDir),
		fmt.Sprintf("docker rm -f %s", credgen),
	}, " && ")
	if err := s.ssh("generate production credentials", s.priv(cmd)); err != nil {
		return err
	}
	done("Production credentials persisted under " + credsDir)
	return nil
}

// syncMasterKeyLocal saves the production master key generated on the remote
// into ./.ric/keys/production.key (mode 0600) on the developer's machine, so
// it can be recovered after a disk wipe / new clone. The file on the server is
// root-owned (docker wrote it), so we sudo-cat it through ssh rather than
// scping. Idempotent — the same content gets overwritten on subsequent runs.
func (s *deploySession) syncMasterKeyLocal() error {
	step("Saving production master key locally")
	keyPath := s.root() + "/credentials/production.key"
	out, ok := s.sshQuiet(s.priv("cat " + shellQuote(keyPath)))
	if !ok || out == "" {
		return fmt.Errorf("could not read %s on %s — key generation may have failed", keyPath, s.cfg.Host)
	}

	const localDir = ".ric/keys"
	if err := os.MkdirAll(localDir, 0700); err != nil {
		return fmt.Errorf("create %s: %w", localDir, err)
	}
	localPath := filepath.Join(localDir, "production.key")
	if err := os.WriteFile(localPath, []byte(out+"\n"), 0600); err != nil {
		return fmt.Errorf("write %s: %w", localPath, err)
	}
	done(localPath + " (mode 0600) — add .ric/keys/ to .gitignore")
	return nil
}

func (s *deploySession) runDBPrepare(sha string) error {
	step("Running db:prepare (migrations + seeds for fresh DBs)")
	cmd := fmt.Sprintf("docker run --rm --network ric-net %s %s:%s bin/rails db:prepare",
		s.appMounts(), s.name, sha)
	if err := s.ssh("db:prepare", s.priv(cmd)); err != nil {
		return fmt.Errorf("db:prepare failed on %s — see rails output above. Common causes: bad migration, missing gem, or insufficient permissions on %s/storage: %w",
			s.cfg.Host, s.root(), err)
	}
	done("db:prepare complete")
	return nil
}

func (s *deploySession) startApp(sha string) error {
	step("Starting application container (rails server + in-puma solid_queue)")
	cmd := fmt.Sprintf(
		"docker rm -f %[1]s >/dev/null 2>&1; true && "+
			"docker run -d --name %[1]s --network ric-net --restart unless-stopped "+
			"%[4]s%[2]s "+
			"%[1]s:%[3]s bin/rails server -b 0.0.0.0 -p 3000",
		s.name, s.appMounts(), sha, s.resourceFlags())
	if err := s.ssh("start app container", s.priv(cmd)); err != nil {
		return fmt.Errorf("could not start app container %s on %s. Run 'docker logs %s' on the server to see why. Underlying: %w",
			s.name, s.cfg.Host, s.name, err)
	}
	done("App container started")
	return nil
}

// ensureSharedNginx starts the singleton ric-nginx container if it isn't
// already running. The container binds the host's 80/443 and mounts the
// shared sites-enabled + ssl trees, so adding a new project is just a matter
// of dropping a site config file and reloading.
func (s *deploySession) ensureSharedNginx() error {
	if out, ok := s.sshQuiet("docker inspect -f '{{.State.Running}}' ric-nginx 2>/dev/null"); ok && out == "true" {
		return nil
	}

	step("Pulling nginx image on remote")
	if err := s.ssh("docker pull nginx:stable", s.priv("docker pull nginx:stable")); err != nil {
		return err
	}
	done("Nginx image ready")

	step("Starting shared ric-nginx container")
	cmd := fmt.Sprintf(
		"docker rm -f ric-nginx >/dev/null 2>&1; true && "+
			"docker run -d --name ric-nginx --network ric-net --restart unless-stopped "+
			"-p 80:80 -p 443:443 "+
			"-v %[1]s/ssl:/etc/nginx/ssl:ro "+
			"-v %[1]s/nginx/sites-enabled:/etc/nginx/conf.d:ro "+
			"nginx:stable",
		sharedRoot)
	if err := s.ssh("start ric-nginx", s.priv(cmd)); err != nil {
		return fmt.Errorf("ric-nginx failed to start on %s — ports 80/443 may already be in use (run 'ss -tlnp | grep -E \":80|:443\"' on the server to check): %w",
			s.cfg.Host, err)
	}
	done("Shared ric-nginx running")
	return nil
}

// reloadNginx asks the running ric-nginx to re-read its config. Uses
// nginx -s reload rather than restarting the container so other projects'
// active connections aren't disrupted. If reload fails, nginx keeps the
// previous (working) config — safer than a container restart.
func (s *deploySession) reloadNginx() error {
	step("Reloading nginx configuration")
	cmd := "docker exec ric-nginx nginx -t && docker exec ric-nginx nginx -s reload"
	if err := s.ssh("nginx reload", s.priv(cmd)); err != nil {
		return fmt.Errorf("nginx -t / reload failed on %s — site config at %s/nginx/sites-enabled/%s.conf may have a syntax error: %w",
			s.cfg.Host, sharedRoot, s.name, err)
	}
	done("Nginx reloaded")
	return nil
}

func (s *deploySession) writeCurrentSHA(sha string) error {
	cmd := fmt.Sprintf("printf %%s %s > %s/current_sha", shellQuote(sha), s.root())
	return s.ssh("record current_sha", cmd)
}

// --- nginx template ---

func renderNginxConf(name, domain string) string {
	return fmt.Sprintf(`server {
    listen 80;
    server_name %[2]s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    http2 on;
    server_name %[2]s;

    ssl_certificate     /etc/nginx/ssl/%[1]s/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/%[1]s/privkey.pem;

    client_max_body_size 50m;

    location / {
        proxy_pass http://%[1]s:3000;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
    }
}
`, name, domain)
}

// --- small helpers ---

func step(msg string) { fmt.Printf("→ %s…\n", msg) }
func done(msg string) { fmt.Printf("  ✓ %s\n", msg) }

// shellQuote wraps a string in single quotes, escaping any embedded single
// quotes. Suitable for paths we synthesize ourselves (no untrusted input).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// registryMirrorURL coerces a user-provided mirror value into the URL form
// docker daemon.json expects ("https://<host>"). Trims whitespace and path,
// adds https:// if no scheme is given.
func registryMirrorURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return s
}

func init() {
	dispatcher.Register("deploy", Deploy)
}
