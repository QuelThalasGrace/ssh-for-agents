package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "0.1.0"

type Config struct {
	Alias     string `json:"alias"`
	Host      string `json:"host"`
	User      string `json:"user"`
	Port      string `json:"port"`
	RemoteDir string `json:"remote_dir"`
	RunPrefix string `json:"run_prefix"`
	KeyPath   string `json:"key_path"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "init":
		initCmd(os.Args[2:])
	case "run":
		runCmd(os.Args[2:])
	case "bg-run":
		bgRunCmd(os.Args[2:])
	case "log":
		logCmd(os.Args[2:])
	case "status", "test":
		statusCmd()
	case "doctor":
		doctorCmd()
	case "uninstall":
		uninstallCmd(os.Args[2:])
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Println("Unknown command:", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf(`ssh-for-agents %s

Usage:
  sfa init --alias ALIAS --host HOST --user USER --port PORT --remote-dir DIR [--identity-file FILE] [--run-prefix CMD]
  sfa run "COMMAND"
  sfa bg-run JOB "COMMAND"
  sfa log logs/JOB.log [N]
  sfa status
  sfa doctor
  sfa uninstall [--all]

`, version)
}

func arg(args []string, name, def string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return def
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func run(name string, args ...string) error {
	fmt.Println("+", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func needCmd(name string) {
	if _, err := exec.LookPath(name); err != nil {
		fmt.Printf("Missing required command: %s\n", name)
		os.Exit(1)
	}
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func configPath() string {
	return filepath.Join(".agent", "config.json")
}

func loadConfig() Config {
	b, err := os.ReadFile(configPath())
	if err != nil {
		fmt.Println("This directory is not initialized. Run sfa init first.")
		os.Exit(1)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		fmt.Println("Invalid .agent/config.json:", err)
		os.Exit(1)
	}
	return cfg
}

func saveConfig(cfg Config) {
	os.MkdirAll(".agent", 0755)
	b, _ := json.MarshalIndent(cfg, "", "  ")
	must(os.WriteFile(configPath(), b, 0644))
}

func initCmd(args []string) {
	alias := arg(args, "--alias", "")
	host := arg(args, "--host", "")
	user := arg(args, "--user", "")
	port := arg(args, "--port", "22")
	remoteDir := arg(args, "--remote-dir", "")
	identityFile := arg(args, "--identity-file", "")
	runPrefix := arg(args, "--run-prefix", "")

	if alias == "" || host == "" || user == "" || remoteDir == "" {
		usage()
		os.Exit(1)
	}

	needCmd("ssh")
	needCmd("ssh-keygen")
	needCmd("scp")

	keyPath := identityFile
	if keyPath == "" {
		keyPath = ensureKey(alias)
	}

	appendSSHConfig(alias, host, user, port, keyPath)

	if !testSSH(alias) {
		if identityFile != "" {
			fmt.Println("SSH key login failed with provided identity file.")
			os.Exit(1)
		}
		installPublicKey(host, user, port, keyPath)
	}

	if !testSSH(alias) {
		fmt.Println("SSH setup failed.")
		os.Exit(1)
	}

	cfg := Config{
		Alias:     alias,
		Host:      host,
		User:      user,
		Port:      port,
		RemoteDir: remoteDir,
		RunPrefix: runPrefix,
		KeyPath:   keyPath,
	}

	must(run("ssh", alias, "mkdir -p "+quote(remoteDir)))
	saveConfig(cfg)
	writeDocs(cfg)
	helloTest(cfg)

	fmt.Println("[ok] ssh-for-agents workspace initialized successfully.")
}

func ensureKey(alias string) string {
	sshDir := filepath.Join(homeDir(), ".ssh")
	must(os.MkdirAll(sshDir, 0700))

	key := filepath.Join(sshDir, alias+"_agent")
	if _, err := os.Stat(key); err == nil {
		if _, err2 := os.Stat(key + ".pub"); err2 == nil {
			return key
		}
	}

	must(run("ssh-keygen", "-t", "ed25519", "-f", key, "-C", alias+"-ssh-for-agents", "-N", ""))
	return key
}

func appendSSHConfig(alias, host, user, port, keyPath string) {
	sshDir := filepath.Join(homeDir(), ".ssh")
	must(os.MkdirAll(sshDir, 0700))

	config := filepath.Join(sshDir, "config")
	existing, _ := os.ReadFile(config)

	if strings.Contains(string(existing), "Host "+alias+"\n") || strings.Contains(string(existing), "Host "+alias+"\r\n") {
		fmt.Println("[warn] Host", alias, "already exists in ~/.ssh/config. Skip writing.")
		return
	}

	block := fmt.Sprintf(`
# Added by ssh-for-agents
Host %s
    HostName %s
    User %s
    Port %s
    IdentityFile %s
    IdentitiesOnly yes
    ServerAliveInterval 30
    ServerAliveCountMax 3
    TCPKeepAlive yes
`, alias, host, user, port, keyPath)

	f, err := os.OpenFile(config, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	must(err)
	defer f.Close()
	_, err = f.WriteString(block)
	must(err)
}

func testSSH(alias string) bool {
	out, err := output("ssh", "-o", "BatchMode=yes", alias, "echo SFA_SSH_OK")
	return err == nil && strings.Contains(out, "SFA_SSH_OK")
}

func installPublicKey(host, user, port, keyPath string) {
	pub, err := os.ReadFile(keyPath + ".pub")
	must(err)

	fmt.Println("[info] Installing public key. Type the server password only in the terminal prompt.")
	remote := "mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"

	cmd := exec.Command("ssh", "-p", port, user+"@"+host, remote)
	cmd.Stdin = strings.NewReader(string(pub))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Run())
}

func writeDocs(cfg Config) {
	agents := fmt.Sprintf(`# ssh-for-agents Instructions

This project uses ssh-for-agents.

Remote runnable code mirror:
%s:%s

The local directory is the source of truth for code.
The remote directory is a runnable code mirror.

Only code and project configuration files are synced.
Runtime artifacts such as logs, outputs, checkpoints, pids, data, and datasets are not synchronized and do not need to match the local directory.

## Workflow

Edit files locally in this directory.

Do not run project code locally unless explicitly requested.

Run foreground commands remotely with:

sfa run "<command>"

Run long jobs remotely with:

sfa bg-run <job-name> "<command>"

View logs with:

sfa log logs/<job-name>.log

Check remote status with:

sfa status

After modifying code, run the appropriate remote command and verify the result.

Never ask the user to paste passwords into chat.
If password auth is needed, the user should type it only into the terminal prompt.

## Safety

sfa uninstall never modifies SSH config files, SSH keys, or remote server files.
`, cfg.Alias, cfg.RemoteDir)

	must(os.WriteFile("AGENTS.md", []byte(agents), 0644))
	must(os.WriteFile("CLAUDE.md", []byte(agents), 0644))

	ctx := fmt.Sprintf(`# Project Context

This project uses ssh-for-agents.

Remote host:
%s

Remote directory:
%s

Run prefix:
%s

Describe the project language, environment, dependencies, and common commands here.
`, cfg.Alias, cfg.RemoteDir, emptyText(cfg.RunPrefix, "(none)"))
	must(os.WriteFile("PROJECT_CONTEXT.md", []byte(ctx), 0644))
}

func helloTest(cfg Config) {
	hello := ".sfa_hello.sh"
	content := `#!/bin/sh
echo "ssh-for-agents remote hello"
echo "host=$(hostname)"
echo "user=$(whoami)"
echo "pwd=$(pwd)"
`
	must(os.WriteFile(hello, []byte(content), 0755))
	runCmd([]string{"sh " + hello})
	os.Remove(hello)
	remoteRun(cfg, "rm -f .sfa_hello.sh")
}

func runCmd(args []string) {
	if len(args) < 1 {
		fmt.Println(`Usage: sfa run "COMMAND"`)
		os.Exit(1)
	}
	cfg := loadConfig()
	cmd := strings.Join(args, " ")
	syncToRemote(cfg)
	remoteRun(cfg, cmd)
}

func bgRunCmd(args []string) {
	if len(args) < 2 {
		fmt.Println(`Usage: sfa bg-run JOB "COMMAND"`)
		os.Exit(1)
	}

	cfg := loadConfig()
	job := args[0]
	cmd := strings.Join(args[1:], " ")

	syncToRemote(cfg)

	script := fmt.Sprintf(`cd %s
mkdir -p logs pids
nohup sh -lc %s > logs/%s.log 2>&1 &
pid=$!
echo "$pid" > pids/%s.pid
echo "Started background job: %s"
echo "PID file: pids/%s.pid"
echo "Log file: logs/%s.log"`,
		quote(cfg.RemoteDir),
		quote(withPrefix(cfg, cmd)),
		job,
		job,
		job,
		job,
		job,
	)

	must(run("ssh", cfg.Alias, "sh -lc "+quote(script)))
}

func logCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sfa log logs/job.log [N]")
		os.Exit(1)
	}

	cfg := loadConfig()
	n := "100"
	if len(args) >= 2 {
		n = args[1]
	}

	remote := fmt.Sprintf(
		`cd %s && if [ ! -f %s ]; then echo "Log file not found: %s"; find logs -maxdepth 1 -type f 2>/dev/null | sort || true; exit 1; fi; tail -n %s %s`,
		quote(cfg.RemoteDir),
		quote(args[0]),
		args[0],
		n,
		quote(args[0]),
	)

	must(run("ssh", cfg.Alias, "sh -lc "+quote(remote)))
}

func statusCmd() {
	cfg := loadConfig()

	remote := fmt.Sprintf(
		`echo "=== HOST ==="; hostname; pwd; whoami; echo; echo "=== REMOTE DIR ==="; cd %s && pwd; echo; echo "=== BACKGROUND JOBS ==="; if [ -d pids ]; then for pidfile in pids/*.pid; do [ -e "$pidfile" ] || continue; job=$(basename "$pidfile" .pid); pid=$(cat "$pidfile"); if ps -p "$pid" >/dev/null 2>&1; then echo "RUNNING  $job  PID=$pid"; else echo "FINISHED $job  PID=$pid"; fi; done; else echo "No pids directory."; fi; echo; echo "=== REMOTE CODE MIRROR FILES ==="; find . -maxdepth 2 -type f | sort | head -100`,
		quote(cfg.RemoteDir),
	)

	must(run("ssh", cfg.Alias, "sh -lc "+quote(remote)))
}

func doctorCmd() {
	fmt.Println("ssh-for-agents", version)
	fmt.Println("OS:", runtime.GOOS, runtime.GOARCH)
	for _, c := range []string{"ssh", "ssh-keygen", "scp"} {
		if p, err := exec.LookPath(c); err == nil {
			fmt.Println("OK:", c, "->", p)
		} else {
			fmt.Println("MISSING:", c)
		}
	}
}

func uninstallCmd(args []string) {
	all := hasFlag(args, "--all")

	fmt.Println("This will uninstall ssh-for-agents from this user account.")
	fmt.Println()
	fmt.Println("It WILL remove:")
	fmt.Println("  - sfa binary in standard install locations")
	fmt.Println("  - ~/.ssh-for-agents if it exists")
	if all {
		fmt.Println("  - .agent/ in the current directory")
		fmt.Println("  - AGENTS.md, CLAUDE.md, PROJECT_CONTEXT.md in the current directory")
	}
	fmt.Println()
	fmt.Println("It WILL NOT remove or modify:")
	fmt.Println("  - ~/.ssh/config")
	fmt.Println("  - SSH keys under ~/.ssh/")
	fmt.Println("  - remote server files")
	fmt.Println()
	fmt.Print("Continue? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(ans)

	if ans != "y" && ans != "Y" && ans != "yes" && ans != "YES" {
		fmt.Println("Cancelled.")
		return
	}

	removeInstallFiles()

	if all {
		os.RemoveAll(".agent")
		os.Remove("AGENTS.md")
		os.Remove("CLAUDE.md")
		os.Remove("PROJECT_CONTEXT.md")
	}

	fmt.Println("[ok] ssh-for-agents uninstalled.")
	fmt.Println("[note] SSH config entries, SSH keys, and remote server files were not touched.")
}

func removeInstallFiles() {
	home := homeDir()
	paths := []string{
		filepath.Join(home, ".local", "bin", "sfa"),
		filepath.Join(home, ".ssh-for-agents", "bin", "sfa.exe"),
		filepath.Join(home, ".ssh-for-agents", "bin", "sfa"),
		filepath.Join(home, ".ssh-for-agents"),
	}

	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
}

func syncToRemote(cfg Config) {
	files := collectFiles()
	for _, rel := range files {
		local := rel
		remotePath := strings.TrimRight(cfg.RemoteDir, "/") + "/" + filepath.ToSlash(rel)
		remoteParent := pathDir(remotePath)

		must(run("ssh", cfg.Alias, "mkdir -p "+quote(remoteParent)))
		must(run("scp", local, cfg.Alias+":"+remotePath))
	}
}

func collectFiles() []string {
	var files []string

	filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == "." {
			return nil
		}

		rel := filepath.ToSlash(path)

		if d.IsDir() {
			if excludedDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		if excludedFile(rel) {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files
}

func excludedDir(p string) bool {
	names := []string{
		".agent",
		".git",
		".venv",
		"__pycache__",
		"logs",
		"outputs",
		"checkpoints",
		"remote_logs",
		"pids",
		"data",
		"datasets",
	}

	base := filepath.Base(p)
	for _, n := range names {
		if base == n {
			return true
		}
	}
	return false
}

func excludedFile(p string) bool {
	base := filepath.Base(p)

	if base == "AGENTS.md" || base == "CLAUDE.md" || base == "PROJECT_CONTEXT.md" || base == ".DS_Store" {
		return true
	}

	if strings.HasSuffix(base, ".pyc") {
		return true
	}

	return false
}

func remoteRun(cfg Config, cmd string) {
	remoteCmd := withPrefix(cfg, cmd)
	remote := fmt.Sprintf("cd %s && %s", quote(cfg.RemoteDir), remoteCmd)
	must(run("ssh", cfg.Alias, "sh -lc "+quote(remote)))
}

func withPrefix(cfg Config, cmd string) string {
	if strings.TrimSpace(cfg.RunPrefix) == "" {
		return cmd
	}
	return cfg.RunPrefix + " && " + cmd
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func pathDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "."
	}
	return p[:i]
}

func emptyText(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func must(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
