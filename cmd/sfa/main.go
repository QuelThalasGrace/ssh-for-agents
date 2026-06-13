package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const version = "0.3.0"

type RuntimeProfile struct {
	Language  string `json:"language"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	RunPrefix string `json:"run_prefix"`
}

type Config struct {
	Alias     string         `json:"alias"`
	Host      string         `json:"host"`
	User      string         `json:"user"`
	Port      string         `json:"port"`
	RemoteDir string         `json:"remote_dir"`
	RunPrefix string         `json:"run_prefix"`
	KeyPath   string         `json:"key_path"`
	Runtime   RuntimeProfile `json:"runtime"`
}

const (
	sfaDocPath       = "SFA.md"
	codexConfigPath  = ".codex/config.toml"
	claudeMemoryPath = ".claude/CLAUDE.md"

	codexManagedBegin = "# BEGIN ssh-for-agents"
	codexManagedEnd   = "# END ssh-for-agents"
	htmlManagedBegin  = "<!-- BEGIN ssh-for-agents -->"
	htmlManagedEnd    = "<!-- END ssh-for-agents -->"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "init":
		initCmd(os.Args[2:])
	case "sync":
		syncCmd(os.Args[2:])
	case "run":
		runCmd(os.Args[2:])
	case "bg-run":
		bgRunCmd(os.Args[2:])
	case "log":
		logCmd(os.Args[2:])
	case "status", "test":
		statusCmd()
	case "pull":
		pullCmd()
	case "clean-code":
		cleanCodeCmd()
	case "destroy":
		destroyCmd()
	case "env":
		envCmd(os.Args[2:])
	case "doctor":
		doctorCmd()
	case "version":
		versionCmd()
	case "upgrade":
		upgradeCmd()
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
  sfa init --alias ALIAS --host HOST --user USER --port PORT (--remote-dir DIR | --remote-base DIR) [--identity-file FILE] [--run-prefix CMD]
  sfa sync [FILE...]
  sfa run [--sync] "COMMAND"
  sfa bg-run [--sync] JOB "COMMAND"
  sfa log logs/JOB.log [N]
  sfa status
  sfa pull
  sfa clean-code
  sfa destroy

  sfa env detect
  sfa env show
  sfa env set python conda ENV_NAME
  sfa env set go system
  sfa env set node system
  sfa env set rust system
  sfa env set dotnet system
  sfa env set java system
  sfa env set-prefix "PREFIX_COMMAND"
  sfa env clear

  sfa doctor
  sfa version
  sfa upgrade
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
	remoteBase := arg(args, "--remote-base", "")
	identityFile := arg(args, "--identity-file", "")
	runPrefix := arg(args, "--run-prefix", "")

	if remoteDir == "" && remoteBase != "" {
		cwd, _ := os.Getwd()
		localName := filepath.Base(cwd)
		remoteDir = strings.TrimRight(remoteBase, "/") + "/" + localName
	}

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
		Runtime: RuntimeProfile{
			Language:  "",
			Kind:      "",
			Name:      "",
			Version:   "",
			RunPrefix: runPrefix,
		},
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
	writeSFA(cfg)
	writeCodexConfig()
	writeClaudeMemory()
}

func writeSFA(cfg Config) {
	managed := buildSFAManagedContent(cfg)
	block := managedBlock(htmlManagedBegin, htmlManagedEnd, managed)

	existing, err := os.ReadFile(sfaDocPath)
	if err != nil {
		content := "# ssh-for-agents Instructions\n\n" + block + "\n\n" + defaultProjectNotes()
		must(os.WriteFile(sfaDocPath, []byte(content), 0644))
		return
	}

	updated, ok := replaceManagedBlock(string(existing), htmlManagedBegin, htmlManagedEnd, managed)
	if !ok {
		updated = "# ssh-for-agents Instructions\n\n" + block + "\n\n" + strings.TrimLeft(string(existing), "\n")
	}
	must(os.WriteFile(sfaDocPath, []byte(ensureTrailingNewline(updated)), 0644))
}

func buildSFAManagedContent(cfg Config) string {
	runtimeText := "not configured"
	if strings.TrimSpace(cfg.Runtime.Language) != "" {
		runtimeText = fmt.Sprintf("%s / %s / %s", cfg.Runtime.Language, cfg.Runtime.Kind, emptyText(cfg.Runtime.Name, "(none)"))
	}

	return fmt.Sprintf(`This project uses ssh-for-agents.

Remote host:
%s

Remote runnable code mirror:
%s:%s

The local directory is the source of truth for code.
The remote directory is a runnable code mirror.

Only code and project configuration files are synced.
Runtime artifacts such as logs, outputs, checkpoints, pids, data, and datasets are not synchronized and do not need to match the local directory.
Environment files such as .env and .env.* are not synchronized.

Runtime:
%s

Run prefix:
%s

## Workflow

Edit files locally in this directory.

Do not run project code locally unless explicitly requested.

After editing code, sync local files to the remote mirror with:

sfa sync

If only a few files changed, sync those files instead of the whole project:

sfa sync <file> [file...]

Run foreground commands remotely without syncing with:

sfa run "<command>"

Or sync and run in one step with:

sfa run --sync "<command>"

Run long jobs remotely without syncing with:

sfa bg-run <job-name> "<command>"

Or sync and start a long job in one step with:

sfa bg-run --sync <job-name> "<command>"

View logs with:

sfa log logs/<job-name>.log

Check remote status with:

sfa status

Upgrade the local sfa binary and refresh this file with:

sfa upgrade

Pull remote code back to local with:

sfa pull

Clean local and remote code/test files with:

sfa clean-code

## Runtime Environment

Before running project commands, inspect .agent/config.json and this SFA.md for the configured runtime.

If no runtime is configured:
1. Run sfa env detect.
2. If a suitable runtime exists, configure it with sfa env set.
3. If no suitable runtime exists, install/create it using sfa run.
4. Configure it with sfa env set or sfa env set-prefix.
5. Verify with a hello world command through sfa run.

After modifying code, run the appropriate remote command and verify the result.

Never ask the user to paste passwords into chat.
If password auth is needed, the user should type it only into the terminal prompt.

## Safety

sfa uninstall never modifies SSH config files, SSH keys, or remote server files.

sfa destroy deletes the current local project directory and its configured remote directory only after confirmation.
`, cfg.Alias, cfg.Alias, cfg.RemoteDir, runtimeText, emptyText(cfg.RunPrefix, "(none)"))
}

func defaultProjectNotes() string {
	return `## Project Notes

Describe the project language, dependencies, common commands, and experiment notes here.
`
}

func writeCodexConfig() {
	must(os.MkdirAll(filepath.Dir(codexConfigPath), 0755))

	block := managedBlock(codexManagedBegin, codexManagedEnd, `project_doc_fallback_filenames = ["SFA.md"]`)
	existing, err := os.ReadFile(codexConfigPath)
	if err != nil {
		must(os.WriteFile(codexConfigPath, []byte(block+"\n"), 0644))
		return
	}

	content := string(existing)
	if updated, ok := replaceManagedBlock(content, codexManagedBegin, codexManagedEnd, `project_doc_fallback_filenames = ["SFA.md"]`); ok {
		must(os.WriteFile(codexConfigPath, []byte(ensureTrailingNewline(updated)), 0644))
		return
	}

	if updated, ok := updateFallbackLine(content, true); ok {
		must(os.WriteFile(codexConfigPath, []byte(ensureTrailingNewline(updated)), 0644))
		return
	}

	updated := block + "\n\n" + strings.TrimLeft(content, "\n")
	must(os.WriteFile(codexConfigPath, []byte(ensureTrailingNewline(updated)), 0644))
}

func writeClaudeMemory() {
	must(os.MkdirAll(filepath.Dir(claudeMemoryPath), 0755))

	existing, err := os.ReadFile(claudeMemoryPath)
	if err != nil || strings.TrimSpace(string(existing)) == "" {
		must(os.WriteFile(claudeMemoryPath, []byte("@../SFA.md\n"), 0644))
		return
	}

	content := string(existing)
	if updated, ok := replaceManagedBlock(content, htmlManagedBegin, htmlManagedEnd, "@../SFA.md"); ok {
		must(os.WriteFile(claudeMemoryPath, []byte(ensureTrailingNewline(updated)), 0644))
		return
	}
	if containsLine(content, "@../SFA.md") {
		return
	}

	block := managedBlock(htmlManagedBegin, htmlManagedEnd, "@../SFA.md")
	updated := strings.TrimRight(content, "\n") + "\n\n" + block + "\n"
	must(os.WriteFile(claudeMemoryPath, []byte(updated), 0644))
}

func managedBlock(begin, end, body string) string {
	return begin + "\n" + strings.TrimRight(body, "\n") + "\n" + end
}

func replaceManagedBlock(content, begin, end, body string) (string, bool) {
	start := strings.Index(content, begin)
	if start < 0 {
		return content, false
	}
	endStart := strings.Index(content[start:], end)
	if endStart < 0 {
		return content, false
	}
	endIndex := start + endStart + len(end)
	block := managedBlock(begin, end, body)
	return content[:start] + block + content[endIndex:], true
}

func removeManagedBlock(content, begin, end string) string {
	for {
		start := strings.Index(content, begin)
		if start < 0 {
			return strings.TrimSpace(content) + trailingNewlineIfNeeded(content)
		}
		endStart := strings.Index(content[start:], end)
		if endStart < 0 {
			return content
		}
		endIndex := start + endStart + len(end)
		content = strings.TrimRight(content[:start], "\n") + "\n" + strings.TrimLeft(content[endIndex:], "\n")
	}
}

func updateFallbackLine(content string, add bool) (string, bool) {
	lines := strings.SplitAfter(content, "\n")
	found := false
	for i, line := range lines {
		updated, ok := updateFallbackLineText(line, add)
		if ok {
			lines[i] = updated
			found = true
		}
	}
	if !found {
		return content, false
	}
	return strings.Join(lines, ""), true
}

func updateFallbackLineText(line string, add bool) (string, bool) {
	lineNoNewline := strings.TrimRight(line, "\n")
	newline := line[len(lineNoNewline):]
	trimmed := strings.TrimSpace(lineNoNewline)
	if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "project_doc_fallback_filenames") {
		return line, false
	}

	open := strings.Index(lineNoNewline, "[")
	close := strings.LastIndex(lineNoNewline, "]")
	if open < 0 || close < open {
		return line, false
	}

	values := parseQuotedList(lineNoNewline[open+1 : close])
	hasSFA := false
	for _, value := range values {
		if value == sfaDocPath {
			hasSFA = true
			break
		}
	}

	if add {
		if !hasSFA {
			values = append(values, sfaDocPath)
		}
	} else {
		var kept []string
		for _, value := range values {
			if value != sfaDocPath {
				kept = append(kept, value)
			}
		}
		values = kept
		if hasSFA && len(values) == 0 {
			return "", true
		}
	}

	updated := lineNoNewline[:open+1] + formatQuotedList(values) + lineNoNewline[close:] + newline
	return updated, true
}

func parseQuotedList(s string) []string {
	var values []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"'`)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func formatQuotedList(values []string) string {
	var quoted []string
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

func containsLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func removeLine(content, want string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func ensureTrailingNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func trailingNewlineIfNeeded(original string) string {
	if strings.TrimSpace(original) == "" {
		return ""
	}
	return "\n"
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
	runCmd([]string{"--sync", "sh " + hello})
	os.Remove(hello)
	remoteRun(cfg, "rm -f .sfa_hello.sh")
}

func syncCmd(args []string) {
	cfg := loadConfig()
	files, err := selectSyncFiles(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	n := syncFilesToRemote(cfg, files)
	fmt.Printf("[ok] synced %d files to remote code mirror.\n", n)
}

func runCmd(args []string) {
	syncFirst, cmd, ok := parseRunArgs(args)
	if !ok {
		fmt.Println(`Usage: sfa run [--sync] "COMMAND"`)
		os.Exit(1)
	}
	cfg := loadConfig()
	if syncFirst {
		syncToRemote(cfg)
	}
	remoteRun(cfg, cmd)
}

func bgRunCmd(args []string) {
	syncFirst, job, cmd, ok := parseBgRunArgs(args)
	if !ok {
		fmt.Println(`Usage: sfa bg-run [--sync] JOB "COMMAND"`)
		os.Exit(1)
	}

	cfg := loadConfig()
	if syncFirst {
		syncToRemote(cfg)
	}

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

func parseRunArgs(args []string) (bool, string, bool) {
	if len(args) < 1 {
		return false, "", false
	}

	syncFirst := false
	if args[0] == "--sync" {
		syncFirst = true
		args = args[1:]
	}

	if len(args) < 1 {
		return syncFirst, "", false
	}

	return syncFirst, strings.Join(args, " "), true
}

func parseBgRunArgs(args []string) (bool, string, string, bool) {
	if len(args) < 1 {
		return false, "", "", false
	}

	syncFirst := false
	if args[0] == "--sync" {
		syncFirst = true
		args = args[1:]
	}

	if len(args) < 2 {
		return syncFirst, "", "", false
	}

	return syncFirst, args[0], strings.Join(args[1:], " "), true
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
		`echo "=== HOST ==="; hostname; pwd; whoami; echo; echo "=== REMOTE DIR ==="; cd %s && pwd; echo; echo "=== BACKGROUND JOBS ==="; if [ -d pids ]; then for pidfile in pids/*.pid; do [ -e "$pidfile" ] || continue; job=$(basename "$pidfile" .pid); pid=$(cat "$pidfile"); if ps -p "$pid" >/dev/null 2>&1; then echo "RUNNING  $job  PID=$pid"; else echo "FINISHED $job  PID=$pid"; fi; done; else echo "No pids directory."; fi; echo; echo "=== RUNTIME ==="; echo %s; echo; echo "=== REMOTE CODE MIRROR FILES ==="; find . -maxdepth 2 -type f | sort | head -100`,
		quote(cfg.RemoteDir),
		quote(emptyText(cfg.RunPrefix, "none")),
	)

	must(run("ssh", cfg.Alias, "sh -lc "+quote(remote)))
}

func envCmd(args []string) {
	if len(args) < 1 {
		envUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "detect":
		envDetectCmd()
	case "show":
		envShowCmd()
	case "set":
		envSetCmd(args[1:])
	case "set-prefix":
		envSetPrefixCmd(args[1:])
	case "clear":
		envClearCmd()
	default:
		envUsage()
		os.Exit(1)
	}
}

func envUsage() {
	fmt.Printf(`Usage:
  sfa env detect
  sfa env show
  sfa env set python conda ENV_NAME
  sfa env set go system
  sfa env set node system
  sfa env set rust system
  sfa env set dotnet system
  sfa env set java system
  sfa env set-prefix "PREFIX_COMMAND"
  sfa env clear
`)
}

func envDetectCmd() {
	cfg := loadConfig()

	script := `
echo "=== Python ==="
command -v conda || true
conda env list 2>/dev/null || true
command -v python || true
python --version 2>&1 || true
command -v python3 || true
python3 --version 2>&1 || true

echo
echo "=== Go ==="
command -v go || true
go version 2>&1 || true

echo
echo "=== Node.js ==="
command -v node || true
node -v 2>&1 || true
command -v npm || true
npm -v 2>&1 || true

echo
echo "=== Rust ==="
command -v rustc || true
rustc --version 2>&1 || true
command -v cargo || true
cargo --version 2>&1 || true

echo
echo "=== .NET / C# ==="
command -v dotnet || true
dotnet --info 2>&1 | head -80 || true
dotnet --list-sdks 2>&1 || true

echo
echo "=== Java ==="
command -v java || true
java -version 2>&1 || true
command -v javac || true
javac -version 2>&1 || true

echo
echo "=== C/C++ ==="
command -v gcc || true
gcc --version 2>&1 | head -2 || true
command -v g++ || true
g++ --version 2>&1 | head -2 || true
command -v cmake || true
cmake --version 2>&1 | head -2 || true
`

	must(run("ssh", cfg.Alias, "sh -lc "+quote(script)))
}

func envShowCmd() {
	cfg := loadConfig()

	fmt.Println("Runtime configuration")
	fmt.Println()
	fmt.Println("Remote:", cfg.Alias+":"+cfg.RemoteDir)
	fmt.Println("Language:", emptyText(cfg.Runtime.Language, "(not configured)"))
	fmt.Println("Kind:", emptyText(cfg.Runtime.Kind, "(not configured)"))
	fmt.Println("Name:", emptyText(cfg.Runtime.Name, "(not configured)"))
	fmt.Println("Version:", emptyText(cfg.Runtime.Version, "(not configured)"))
	fmt.Println("Run prefix:", emptyText(cfg.RunPrefix, "(none)"))
}

func envSetCmd(args []string) {
	if len(args) < 2 {
		envUsage()
		os.Exit(1)
	}

	cfg := loadConfig()
	language := strings.ToLower(args[0])
	kind := strings.ToLower(args[1])

	switch language {
	case "python":
		if kind != "conda" {
			fmt.Println("Currently supported Python runtime kind: conda")
			os.Exit(1)
		}
		if len(args) < 3 {
			fmt.Println("Usage: sfa env set python conda ENV_NAME")
			os.Exit(1)
		}
		envName := args[2]
		prefix := detectCondaRunPrefix(cfg, envName)
		cfg.RunPrefix = prefix
		cfg.Runtime = RuntimeProfile{
			Language:  "python",
			Kind:      "conda",
			Name:      envName,
			Version:   "",
			RunPrefix: prefix,
		}

	case "go", "node", "rust", "dotnet", "java":
		if kind != "system" {
			fmt.Printf("Currently supported %s runtime kind: system\n", language)
			os.Exit(1)
		}
		cfg.RunPrefix = ""
		cfg.Runtime = RuntimeProfile{
			Language:  language,
			Kind:      "system",
			Name:      "",
			Version:   "",
			RunPrefix: "",
		}

	default:
		fmt.Println("Unsupported language:", language)
		envUsage()
		os.Exit(1)
	}

	saveConfig(cfg)
	writeSFA(cfg)

	fmt.Println("[ok] runtime configured.")
	envShowCmd()
}

func detectCondaRunPrefix(cfg Config, envName string) string {
	script := `
if command -v conda >/dev/null 2>&1; then
  conda info --base 2>/dev/null
fi
`
	out, err := output("ssh", cfg.Alias, "sh -lc "+quote(script))
	if err != nil {
		fmt.Print(out)
		fmt.Println("Failed to detect conda base. Use sfa env set-prefix instead.")
		os.Exit(1)
	}

	base := strings.TrimSpace(out)
	if base == "" {
		fmt.Println("Conda was not found on the remote server. Create/install conda first, or use sfa env set-prefix.")
		os.Exit(1)
	}

	condaSH := strings.TrimRight(base, "/") + "/etc/profile.d/conda.sh"
	return ". " + quote(condaSH) + " && conda activate " + shellToken(envName)
}

func envSetPrefixCmd(args []string) {
	if len(args) < 1 {
		fmt.Println(`Usage: sfa env set-prefix "PREFIX_COMMAND"`)
		os.Exit(1)
	}

	cfg := loadConfig()
	prefix := strings.Join(args, " ")

	cfg.RunPrefix = prefix
	cfg.Runtime = RuntimeProfile{
		Language:  "custom",
		Kind:      "prefix",
		Name:      "custom",
		Version:   "",
		RunPrefix: prefix,
	}

	saveConfig(cfg)
	writeSFA(cfg)

	fmt.Println("[ok] custom runtime prefix configured.")
	envShowCmd()
}

func envClearCmd() {
	cfg := loadConfig()

	cfg.RunPrefix = ""
	cfg.Runtime = RuntimeProfile{}

	saveConfig(cfg)
	writeSFA(cfg)

	fmt.Println("[ok] runtime configuration cleared.")
}

func pullCmd() {
	cfg := loadConfig()

	fmt.Println("This will pull remote code mirror files back into the current local directory.")
	fmt.Println("Local files with the same paths may be overwritten.")
	fmt.Println()
	fmt.Println("It will NOT pull logs, outputs, checkpoints, pids, data, datasets, .env files, .agent, .codex, .claude, SFA.md, or legacy agent docs.")
	fmt.Println()
	if !confirm("Continue? [y/N] ") {
		fmt.Println("Cancelled.")
		return
	}

	files := remoteListFiles(cfg)
	for _, rel := range files {
		rel = strings.TrimPrefix(rel, "./")
		localPath := filepath.FromSlash(rel)
		parent := filepath.Dir(localPath)
		if parent != "." {
			must(os.MkdirAll(parent, 0755))
		}
		remotePath := strings.TrimRight(cfg.RemoteDir, "/") + "/" + filepath.ToSlash(rel)
		must(run("scp", cfg.Alias+":"+remotePath, localPath))
	}

	fmt.Printf("[ok] pulled %d files from remote code mirror.\n", len(files))
}

func cleanCodeCmd() {
	cfg := loadConfig()

	fmt.Println("This will remove syncable code/test files from both local and remote workspaces.")
	fmt.Println()
	fmt.Println("It WILL remove:")
	fmt.Println("  - local and remote syncable code files")
	fmt.Println("  - local and remote logs/")
	fmt.Println("  - local and remote outputs/")
	fmt.Println("  - local and remote pids/")
	fmt.Println()
	fmt.Println("It WILL NOT remove:")
	fmt.Println("  - .agent/")
	fmt.Println("  - .codex/ and .claude/")
	fmt.Println("  - SFA.md and legacy agent docs")
	fmt.Println("  - .git/")
	fmt.Println("  - checkpoints/")
	fmt.Println("  - data/")
	fmt.Println("  - datasets/")
	fmt.Println("  - SSH config, SSH keys, or unrelated remote files")
	fmt.Println()

	if !confirm("Continue? [y/N] ") {
		fmt.Println("Cancelled.")
		return
	}

	localFiles := collectFiles()
	for _, f := range localFiles {
		_ = os.Remove(f)
	}
	for _, d := range []string{"logs", "outputs", "pids"} {
		_ = os.RemoveAll(d)
	}

	remoteFiles := remoteListFiles(cfg)
	for _, rel := range remoteFiles {
		rel = strings.TrimPrefix(rel, "./")
		remotePath := strings.TrimRight(cfg.RemoteDir, "/") + "/" + filepath.ToSlash(rel)
		must(run("ssh", cfg.Alias, "rm -f "+quote(remotePath)))
	}

	remoteClean := fmt.Sprintf("rm -rf %s %s %s",
		quote(strings.TrimRight(cfg.RemoteDir, "/")+"/logs"),
		quote(strings.TrimRight(cfg.RemoteDir, "/")+"/outputs"),
		quote(strings.TrimRight(cfg.RemoteDir, "/")+"/pids"),
	)
	must(run("ssh", cfg.Alias, remoteClean))

	fmt.Printf("[ok] removed %d local files and %d remote files, plus logs/ outputs/ pids/.\n", len(localFiles), len(remoteFiles))
}

func destroyCmd() {
	cfg := loadConfig()
	cwd, err := os.Getwd()
	must(err)
	projectName := filepath.Base(cwd)

	fmt.Println("DANGER: This will delete the entire project.")
	fmt.Println()
	fmt.Println("It WILL remove:")
	fmt.Println("  - current local project directory:", cwd)
	fmt.Println("  - configured remote directory:", cfg.Alias+":"+cfg.RemoteDir)
	fmt.Println()
	fmt.Println("It WILL NOT remove or modify:")
	fmt.Println("  - ~/.ssh/config")
	fmt.Println("  - SSH keys")
	fmt.Println("  - GitHub repositories")
	fmt.Println("  - unrelated remote files")
	fmt.Println()
	fmt.Printf("Type the local project directory name to confirm [%s]: ", projectName)

	reader := bufio.NewReader(os.Stdin)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(ans)

	if ans != projectName {
		fmt.Println("Cancelled.")
		return
	}

	must(run("ssh", cfg.Alias, "rm -rf "+quote(cfg.RemoteDir)))
	parent := filepath.Dir(cwd)

	if err := os.Chdir(parent); err != nil {
		fmt.Println("Warning: failed to move out of project directory:", err)
	}

	must(os.RemoveAll(cwd))

	fmt.Println("[ok] local and remote project directories removed.")
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

	if _, err := os.Stat(configPath()); err == nil {
		cfg := loadConfig()
		fmt.Println()
		fmt.Println("Project initialized: yes")
		fmt.Println("Remote:", cfg.Alias+":"+cfg.RemoteDir)
		if strings.TrimSpace(cfg.RunPrefix) != "" {
			fmt.Println("Run prefix:", cfg.RunPrefix)
		} else {
			fmt.Println("Run prefix: none")
		}
		if strings.TrimSpace(cfg.Runtime.Language) != "" {
			fmt.Println("Runtime:", cfg.Runtime.Language, cfg.Runtime.Kind, cfg.Runtime.Name)
		} else {
			fmt.Println("Runtime: not configured")
		}
	} else {
		fmt.Println()
		fmt.Println("Project initialized: no")
	}
}

func versionCmd() {
	fmt.Println("ssh-for-agents", version)
}

func upgradeCmd() {
	asset := upgradeAssetName(runtime.GOOS, runtime.GOARCH)
	if asset == "" {
		fmt.Printf("Unsupported platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(1)
	}

	baseURL := os.Getenv("SFA_BASE_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download"
	}
	url := upgradeDownloadURL(baseURL, asset)

	current, err := os.Executable()
	must(err)
	current, err = filepath.EvalSymlinks(current)
	if err != nil {
		current, _ = os.Executable()
	}

	tmp, err := downloadUpgrade(url)
	must(err)
	must(installUpgrade(tmp, current))

	fmt.Println("[ok] upgraded sfa binary:", current)
	if refreshProjectDocsIfInitialized() {
		fmt.Println("[ok] refreshed SFA.md and agent adapter files.")
	} else {
		fmt.Println("[note] current directory is not an initialized sfa project; skipped SFA.md refresh.")
	}
}

func upgradeAssetName(goos, goarch string) string {
	var osName string
	switch goos {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		return ""
	}

	var archName string
	switch goarch {
	case "arm64", "aarch64":
		archName = "arm64"
	case "amd64", "x86_64":
		archName = "amd64"
	default:
		return ""
	}

	name := "sfa-" + osName + "-" + archName
	if osName == "windows" {
		name += ".exe"
	}
	return name
}

func upgradeDownloadURL(baseURL, asset string) string {
	return strings.TrimRight(baseURL, "/") + "/" + asset
}

func downloadUpgrade(url string) (string, error) {
	fmt.Println("[info] downloading:", url)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: HTTP %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "sfa-upgrade-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	n, err := tmp.ReadFrom(resp.Body)
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if n == 0 {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("downloaded file is empty")
	}
	return tmp.Name(), nil
}

func installUpgrade(tmp, target string) error {
	if runtime.GOOS == "windows" {
		return installUpgradeWindows(tmp, target)
	}

	if err := os.Chmod(tmp, 0755); err != nil {
		return err
	}
	backup := target + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func installUpgradeWindows(tmp, target string) error {
	dir := filepath.Dir(target)
	pending := filepath.Join(dir, "sfa-upgrade-pending.exe")
	if err := copyFile(tmp, pending, 0755); err != nil {
		return err
	}
	_ = os.Remove(tmp)

	script := filepath.Join(os.TempDir(), fmt.Sprintf("sfa-upgrade-%d.cmd", time.Now().UnixNano()))
	content := fmt.Sprintf(`@echo off
ping 127.0.0.1 -n 2 > nul
move /Y %q %q > nul
del %%~f0
`, pending, target)
	if err := os.WriteFile(script, []byte(content), 0600); err != nil {
		return err
	}
	return exec.Command("cmd", "/C", "start", "", script).Start()
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Close()
}

func refreshProjectDocsIfInitialized() bool {
	cfg, ok := tryLoadConfig()
	if !ok {
		return false
	}
	writeDocs(cfg)
	return true
}

func tryLoadConfig() (Config, bool) {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}, false
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, false
	}
	return cfg, true
}

func uninstallCmd(args []string) {
	all := hasFlag(args, "--all")

	fmt.Println("This will uninstall ssh-for-agents from this user account.")
	fmt.Println()
	fmt.Println("It WILL remove:")
	fmt.Println("  - sfa binary in standard install locations")
	fmt.Println("  - ~/.ssh-for-agents if it exists")
	if all {
		fmt.Println("  - .agent/ and SFA.md in the current directory")
		fmt.Println("  - ssh-for-agents entries in .codex/config.toml and .claude/CLAUDE.md")
	}
	fmt.Println()
	fmt.Println("It WILL NOT remove or modify:")
	fmt.Println("  - ~/.ssh/config")
	fmt.Println("  - SSH keys under ~/.ssh/")
	fmt.Println("  - remote server files")
	fmt.Println()
	if !confirm("Continue? [y/N] ") {
		fmt.Println("Cancelled.")
		return
	}

	removeInstallFiles()

	if all {
		cleanupProjectFiles()
	}

	fmt.Println("[ok] ssh-for-agents uninstalled.")
	fmt.Println("[note] SSH config entries, SSH keys, and remote server files were not touched.")
}

func cleanupProjectFiles() {
	os.RemoveAll(".agent")
	os.Remove(sfaDocPath)
	removeLegacyGeneratedDocs()
	cleanupCodexConfig()
	cleanupClaudeMemory()
}

func removeLegacyGeneratedDocs() {
	legacy := map[string]string{
		"AGENTS.md":          "# ssh-for-agents Instructions",
		"CLAUDE.md":          "# ssh-for-agents Instructions",
		"PROJECT_CONTEXT.md": "# Project Context",
	}
	for path, marker := range legacy {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), marker) && strings.Contains(string(b), "ssh-for-agents") {
			os.Remove(path)
		}
	}
}

func cleanupCodexConfig() {
	b, err := os.ReadFile(codexConfigPath)
	if err != nil {
		return
	}

	content := removeManagedBlock(string(b), codexManagedBegin, codexManagedEnd)
	if updated, ok := updateFallbackLine(content, false); ok {
		content = updated
	}
	writeOrRemoveEmpty(codexConfigPath, content)
	removeEmptyDir(filepath.Dir(codexConfigPath))
}

func cleanupClaudeMemory() {
	b, err := os.ReadFile(claudeMemoryPath)
	if err != nil {
		return
	}

	content := removeManagedBlock(string(b), htmlManagedBegin, htmlManagedEnd)
	content = removeLine(content, "@../SFA.md")
	writeOrRemoveEmpty(claudeMemoryPath, content)
	removeEmptyDir(filepath.Dir(claudeMemoryPath))
}

func writeOrRemoveEmpty(path, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		os.Remove(path)
		return
	}
	must(os.WriteFile(path, []byte(content+"\n"), 0644))
}

func removeEmptyDir(path string) {
	_ = os.Remove(path)
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

func syncToRemote(cfg Config) int {
	files, err := selectSyncFiles(nil)
	must(err)
	return syncFilesToRemote(cfg, files)
}

func syncFilesToRemote(cfg Config, files []string) int {
	for _, rel := range files {
		local := rel
		remotePath := strings.TrimRight(cfg.RemoteDir, "/") + "/" + filepath.ToSlash(rel)
		remoteParent := pathDir(remotePath)

		must(run("ssh", cfg.Alias, "mkdir -p "+quote(remoteParent)))
		must(run("scp", local, cfg.Alias+":"+remotePath))
	}
	return len(files)
}

func selectSyncFiles(args []string) ([]string, error) {
	if len(args) == 0 {
		return collectFiles(), nil
	}

	var files []string
	seen := map[string]bool{}
	for _, arg := range args {
		rel, err := validateSyncFile(arg)
		if err != nil {
			return nil, err
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		files = append(files, rel)
	}
	return files, nil
}

func validateSyncFile(arg string) (string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", fmt.Errorf("sync target is empty")
	}

	clean := filepath.Clean(arg)
	if filepath.IsAbs(clean) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(cwd, clean)
		if err != nil {
			return "", err
		}
		clean = rel
	}

	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sync target must stay inside the current project: %s", arg)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("sync target not found: %s", arg)
	}
	if info.IsDir() {
		return "", fmt.Errorf("sync target is a directory, not a file: %s", arg)
	}

	rel := filepath.ToSlash(clean)
	if excludedRemoteFile(rel) {
		return "", fmt.Errorf("sync target is excluded: %s", arg)
	}
	return filepath.FromSlash(rel), nil
}

func remoteListFiles(cfg Config) []string {
	findCmd := fmt.Sprintf(
		`cd %s && find . -type f -print`,
		quote(cfg.RemoteDir),
	)

	out, err := output("ssh", cfg.Alias, "sh -lc "+quote(findCmd))
	if err != nil {
		fmt.Print(out)
		must(err)
	}

	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		rel := strings.TrimPrefix(line, "./")
		if excludedRemoteFile(rel) {
			continue
		}

		files = append(files, line)
	}
	return files
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
		".codex",
		".claude",
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

	if base == "SFA.md" || base == "AGENTS.md" || base == "CLAUDE.md" || base == "PROJECT_CONTEXT.md" || base == ".DS_Store" || isEnvFile(base) {
		return true
	}

	if strings.HasSuffix(base, ".pyc") {
		return true
	}

	return false
}

func isEnvFile(base string) bool {
	return base == ".env" || strings.HasPrefix(base, ".env.")
}

func excludedRemoteFile(p string) bool {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for _, part := range parts {
		if excludedDir(part) {
			return true
		}
	}
	return excludedFile(filepath.Base(p))
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

func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(ans)
	return ans == "y" || ans == "Y" || ans == "yes" || ans == "YES"
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func shellToken(s string) string {
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
