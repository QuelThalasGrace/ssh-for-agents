package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDocsCreatesSFAAndAgentAdapters(t *testing.T) {
	dir := chdirTemp(t)

	cfg := Config{
		Alias:     "myserver",
		RemoteDir: "/home/ubuntu/my_project",
		RunPrefix: "",
	}

	writeDocs(cfg)

	sfa := mustReadTestFile(t, filepath.Join(dir, "SFA.md"))
	if !strings.Contains(sfa, "Remote runnable code mirror:\nmyserver:/home/ubuntu/my_project") {
		t.Fatalf("SFA.md missing remote mirror details:\n%s", sfa)
	}
	if strings.Contains(sfa, "PROJECT_CONTEXT.md") {
		t.Fatalf("SFA.md still mentions PROJECT_CONTEXT.md:\n%s", sfa)
	}

	codex := mustReadTestFile(t, filepath.Join(dir, ".codex", "config.toml"))
	if !strings.Contains(codex, `project_doc_fallback_filenames = ["SFA.md"]`) {
		t.Fatalf(".codex/config.toml missing SFA fallback:\n%s", codex)
	}

	claude := mustReadTestFile(t, filepath.Join(dir, ".claude", "CLAUDE.md"))
	if claude != "@../SFA.md\n" {
		t.Fatalf(".claude/CLAUDE.md = %q, want @../SFA.md newline", claude)
	}

	for _, old := range []string{"AGENTS.md", "CLAUDE.md", "PROJECT_CONTEXT.md"} {
		if _, err := os.Stat(filepath.Join(dir, old)); !os.IsNotExist(err) {
			t.Fatalf("%s exists after writeDocs, want not generated", old)
		}
	}
}

func TestWriteSFAUpdatesManagedBlockAndPreservesNotes(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "SFA.md")
	initial := `# ssh-for-agents

<!-- BEGIN ssh-for-agents -->
Runtime:
old runtime
<!-- END ssh-for-agents -->

## Project Notes

Keep this note.
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Alias:     "myserver",
		RemoteDir: "/srv/app",
		RunPrefix: "conda activate trading",
		Runtime: RuntimeProfile{
			Language: "python",
			Kind:     "conda",
			Name:     "trading",
		},
	}

	writeSFA(cfg)

	updated := mustReadTestFile(t, path)
	if !strings.Contains(updated, "Runtime:\npython / conda / trading") {
		t.Fatalf("SFA.md missing updated runtime:\n%s", updated)
	}
	if strings.Contains(updated, "old runtime") {
		t.Fatalf("SFA.md retained stale managed runtime:\n%s", updated)
	}
	if !strings.Contains(updated, "Keep this note.") {
		t.Fatalf("SFA.md did not preserve notes:\n%s", updated)
	}
}

func TestWriteCodexConfigAddsSFAFallbackWithoutDroppingExistingFallbacks(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".codex", "config.toml")
	initial := "project_doc_fallback_filenames = ['TEAM.md']\nmodel = \"gpt-5\"\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	writeCodexConfig()

	updated := mustReadTestFile(t, path)
	if !strings.Contains(updated, `project_doc_fallback_filenames = ["TEAM.md", "SFA.md"]`) {
		t.Fatalf(".codex/config.toml did not merge SFA fallback:\n%s", updated)
	}
	if !strings.Contains(updated, `model = "gpt-5"`) {
		t.Fatalf(".codex/config.toml dropped user content:\n%s", updated)
	}
}

func TestWriteCodexConfigDoesNotDuplicateExistingSFAFallback(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".codex", "config.toml")
	initial := "project_doc_fallback_filenames = [\"SFA.md\"]\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	writeCodexConfig()

	updated := mustReadTestFile(t, path)
	if strings.Count(updated, "project_doc_fallback_filenames") != 1 {
		t.Fatalf(".codex/config.toml duplicated fallback config:\n%s", updated)
	}
	if strings.Contains(updated, "BEGIN ssh-for-agents") {
		t.Fatalf(".codex/config.toml added managed block unnecessarily:\n%s", updated)
	}
}

func TestCleanupProjectFilesRemovesOnlySFAManagedContent(t *testing.T) {
	dir := chdirTemp(t)

	if err := os.MkdirAll(filepath.Join(dir, ".agent"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agent", "config.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SFA.md"), []byte("sfa docs\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	codex := `model = "gpt-5"
project_doc_fallback_filenames = ["TEAM.md", "SFA.md"]

# BEGIN ssh-for-agents
project_doc_fallback_filenames = ["SFA.md"]
# END ssh-for-agents
`
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(codex), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	claude := `# Team Claude Notes

<!-- BEGIN ssh-for-agents -->
@../SFA.md
<!-- END ssh-for-agents -->
`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte(claude), 0644); err != nil {
		t.Fatal(err)
	}

	cleanupProjectFiles()

	if _, err := os.Stat(filepath.Join(dir, ".agent")); !os.IsNotExist(err) {
		t.Fatalf(".agent exists after cleanup, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SFA.md")); !os.IsNotExist(err) {
		t.Fatalf("SFA.md exists after cleanup, err=%v", err)
	}

	codexUpdated := mustReadTestFile(t, filepath.Join(dir, ".codex", "config.toml"))
	if !strings.Contains(codexUpdated, `model = "gpt-5"`) || !strings.Contains(codexUpdated, `"TEAM.md"`) {
		t.Fatalf(".codex/config.toml did not preserve user content:\n%s", codexUpdated)
	}
	if strings.Contains(codexUpdated, "SFA.md") || strings.Contains(codexUpdated, "ssh-for-agents") {
		t.Fatalf(".codex/config.toml retained sfa content:\n%s", codexUpdated)
	}

	claudeUpdated := mustReadTestFile(t, filepath.Join(dir, ".claude", "CLAUDE.md"))
	if !strings.Contains(claudeUpdated, "# Team Claude Notes") {
		t.Fatalf(".claude/CLAUDE.md did not preserve user notes:\n%s", claudeUpdated)
	}
	if strings.Contains(claudeUpdated, "SFA.md") || strings.Contains(claudeUpdated, "ssh-for-agents") {
		t.Fatalf(".claude/CLAUDE.md retained sfa content:\n%s", claudeUpdated)
	}
}

func TestCleanupProjectFilesRemovesEmptyAdapterFiles(t *testing.T) {
	dir := chdirTemp(t)

	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte("project_doc_fallback_filenames = [\"SFA.md\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte("@../SFA.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cleanupProjectFiles()

	if _, err := os.Stat(filepath.Join(dir, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf(".codex/config.toml exists after cleanup, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex")); !os.IsNotExist(err) {
		t.Fatalf(".codex exists after cleanup, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf(".claude/CLAUDE.md exists after cleanup, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude")); !os.IsNotExist(err) {
		t.Fatalf(".claude exists after cleanup, err=%v", err)
	}
}

func TestUpgradeAssetNameUsesPlatformAndArch(t *testing.T) {
	if got := upgradeAssetName("darwin", "arm64"); got != "sfa-darwin-arm64" {
		t.Fatalf("upgradeAssetName(darwin, arm64) = %q", got)
	}
	if got := upgradeAssetName("linux", "x86_64"); got != "sfa-linux-amd64" {
		t.Fatalf("upgradeAssetName(linux, x86_64) = %q", got)
	}
	if got := upgradeAssetName("windows", "amd64"); got != "sfa-windows-amd64.exe" {
		t.Fatalf("upgradeAssetName(windows, amd64) = %q", got)
	}
	if got := upgradeAssetName("plan9", "amd64"); got != "" {
		t.Fatalf("upgradeAssetName(plan9, amd64) = %q, want empty", got)
	}
}

func TestUpgradeDownloadURLHonorsBaseURL(t *testing.T) {
	got := upgradeDownloadURL("https://example.com/releases/", "sfa-linux-amd64")
	want := "https://example.com/releases/sfa-linux-amd64"
	if got != want {
		t.Fatalf("upgradeDownloadURL = %q, want %q", got, want)
	}
}

func TestRefreshProjectDocsIfInitializedUpdatesSFA(t *testing.T) {
	dir := chdirTemp(t)

	cfg := Config{
		Alias:     "myserver",
		RemoteDir: "/srv/app",
		Runtime: RuntimeProfile{
			Language: "python",
			Kind:     "conda",
			Name:     "trading",
		},
	}
	saveConfig(cfg)
	if err := os.WriteFile(filepath.Join(dir, "SFA.md"), []byte(`# ssh-for-agents

<!-- BEGIN ssh-for-agents -->
Runtime:
old
<!-- END ssh-for-agents -->

## Project Notes

Keep this project note.
`), 0644); err != nil {
		t.Fatal(err)
	}

	if !refreshProjectDocsIfInitialized() {
		t.Fatal("refreshProjectDocsIfInitialized returned false, want true")
	}

	sfa := mustReadTestFile(t, filepath.Join(dir, "SFA.md"))
	if !strings.Contains(sfa, "Runtime:\npython / conda / trading") {
		t.Fatalf("SFA.md was not refreshed:\n%s", sfa)
	}
	if !strings.Contains(sfa, "Keep this project note.") {
		t.Fatalf("SFA.md did not preserve project notes:\n%s", sfa)
	}
}

func TestRefreshProjectDocsIfInitializedSkipsUninitializedDirectory(t *testing.T) {
	chdirTemp(t)

	if refreshProjectDocsIfInitialized() {
		t.Fatal("refreshProjectDocsIfInitialized returned true, want false")
	}
	if _, err := os.Stat("SFA.md"); !os.IsNotExist(err) {
		t.Fatalf("SFA.md exists after skipped refresh, err=%v", err)
	}
}

func TestHelloTestCommandDoesNotReferenceSyncScript(t *testing.T) {
	cmd := helloTestCommand()

	for _, want := range []string{
		`echo "ssh-for-agents remote hello"`,
		`echo "host=$(hostname)"`,
		`echo "user=$(whoami)"`,
		`echo "pwd=$(pwd)"`,
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("helloTestCommand missing %q in:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, ".sfa_hello.sh") {
		t.Fatalf("helloTestCommand still references .sfa_hello.sh:\n%s", cmd)
	}
	if strings.Contains(cmd, "sync") {
		t.Fatalf("helloTestCommand unexpectedly mentions sync:\n%s", cmd)
	}
}

func TestExcludedFileSkipsEnvFiles(t *testing.T) {
	cases := []string{
		".env",
		"app/.env",
		".env.local",
		"app/.env.production",
		"app/.env.development.local",
	}

	for _, path := range cases {
		if !excludedFile(path) {
			t.Fatalf("excludedFile(%q) = false, want true", path)
		}
	}
}

func TestCollectFilesSkipsAgentAdapterFilesInTempProject(t *testing.T) {
	chdirTemp(t)

	mustWriteTestFile(t, "main.go")
	mustWriteTestFile(t, "SFA.md")
	mustWriteTestFile(t, filepath.Join(".codex", "config.toml"))
	mustWriteTestFile(t, filepath.Join(".claude", "CLAUDE.md"))
	mustWriteTestFile(t, "AGENTS.md")
	mustWriteTestFile(t, "CLAUDE.md")
	mustWriteTestFile(t, "PROJECT_CONTEXT.md")

	files := map[string]bool{}
	for _, file := range collectFiles() {
		files[file] = true
	}

	for _, skipped := range []string{
		"SFA.md",
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".claude", "CLAUDE.md"),
		"AGENTS.md",
		"CLAUDE.md",
		"PROJECT_CONTEXT.md",
	} {
		if files[skipped] {
			t.Fatalf("collectFiles included %s", skipped)
		}
	}
	if !files["main.go"] {
		t.Fatal("collectFiles did not include main.go")
	}
}

func TestSelectSyncFilesDefaultsToAllSyncableFiles(t *testing.T) {
	chdirTemp(t)

	mustWriteTestFile(t, "main.go")
	mustWriteTestFile(t, "SFA.md")
	mustWriteTestFile(t, filepath.Join("service", "app.go"))

	files, err := selectSyncFiles(nil)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, file := range files {
		got[file] = true
	}
	if !got["main.go"] {
		t.Fatal("selectSyncFiles(nil) did not include main.go")
	}
	if !got[filepath.Join("service", "app.go")] {
		t.Fatal("selectSyncFiles(nil) did not include service/app.go")
	}
	if got["SFA.md"] {
		t.Fatal("selectSyncFiles(nil) included SFA.md")
	}
}

func TestSelectSyncFilesAcceptsOneOrMoreFiles(t *testing.T) {
	chdirTemp(t)

	mustWriteTestFile(t, "main.go")
	mustWriteTestFile(t, filepath.Join("service", "app.go"))

	files, err := selectSyncFiles([]string{"main.go", filepath.Join("service", "app.go")})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"main.go", filepath.Join("service", "app.go")}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("selectSyncFiles returned %v, want %v", files, want)
	}
}

func TestSelectSyncFilesRejectsUnsafeOrExcludedTargets(t *testing.T) {
	chdirTemp(t)

	mustWriteTestFile(t, "main.go")
	mustWriteTestFile(t, "SFA.md")
	mustWriteTestFile(t, ".env")
	mustWriteTestFile(t, filepath.Join(".agent", "config.json"))
	mustWriteTestFile(t, filepath.Join("logs", "job.log"))

	cases := [][]string{
		{"missing.go"},
		{"."},
		{".."},
		{filepath.Join("..", "outside.go")},
		{"SFA.md"},
		{".env"},
		{filepath.Join(".agent", "config.json")},
		{filepath.Join("logs", "job.log")},
	}

	for _, args := range cases {
		if files, err := selectSyncFiles(args); err == nil {
			t.Fatalf("selectSyncFiles(%v) = %v, nil error; want error", args, files)
		}
	}
}

func TestSelectPullFilesDefaultsToRemoteList(t *testing.T) {
	files, err := selectPullFiles(nil, []string{"./main.go", "./service/app.go"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"./main.go", "./service/app.go"}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("selectPullFiles returned %v, want %v", files, want)
	}
}

func TestSelectPullFilesAcceptsOneOrMoreRemoteFiles(t *testing.T) {
	files, err := selectPullFiles([]string{"main.go", filepath.Join("service", "app.go")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"main.go", filepath.Join("service", "app.go")}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("selectPullFiles returned %v, want %v", files, want)
	}
}

func TestSelectPullFilesRejectsUnsafeOrExcludedTargets(t *testing.T) {
	cases := [][]string{
		{"."},
		{".."},
		{filepath.Join("..", "outside.go")},
		{"/tmp/outside.go"},
		{"SFA.md"},
		{".env"},
		{filepath.Join(".agent", "config.json")},
		{filepath.Join("logs", "job.log")},
	}

	for _, args := range cases {
		if files, err := selectPullFiles(args, nil); err == nil {
			t.Fatalf("selectPullFiles(%v) = %v, nil error; want error", args, files)
		}
	}
}

func TestExcludedRemoteFileSkipsEnvFiles(t *testing.T) {
	cases := []string{
		".env",
		"service/.env.local",
		"service/api/.env.production",
	}

	for _, path := range cases {
		if !excludedRemoteFile(path) {
			t.Fatalf("excludedRemoteFile(%q) = false, want true", path)
		}
	}
}

func TestExcludedFileDoesNotSkipDirenvFile(t *testing.T) {
	if excludedFile(".envrc") {
		t.Fatal("excludedFile(\".envrc\") = true, want false")
	}
}

func TestCollectFilesSkipsEnvFilesInTempProject(t *testing.T) {
	dir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	mustWriteTestFile(t, "main.go")
	mustWriteTestFile(t, ".env")
	mustWriteTestFile(t, ".envrc")
	mustWriteTestFile(t, "service/.env.local")
	mustWriteTestFile(t, "service/app.go")

	files := map[string]bool{}
	for _, file := range collectFiles() {
		files[file] = true
	}

	envLocal := filepath.Join("service", ".env.local")
	appFile := filepath.Join("service", "app.go")

	if files[".env"] {
		t.Fatal("collectFiles included .env")
	}
	if files[envLocal] {
		t.Fatalf("collectFiles included %s", envLocal)
	}
	if !files["main.go"] {
		t.Fatal("collectFiles did not include main.go")
	}
	if !files[appFile] {
		t.Fatalf("collectFiles did not include %s", appFile)
	}
	if !files[".envrc"] {
		t.Fatal("collectFiles did not include .envrc")
	}
}

func TestParseRunArgsDefaultsToNoSync(t *testing.T) {
	syncFirst, cmd, ok := parseRunArgs([]string{"python", "train.py"})
	if !ok {
		t.Fatal("parseRunArgs returned ok=false")
	}
	if syncFirst {
		t.Fatal("parseRunArgs syncFirst = true, want false")
	}
	if cmd != "python train.py" {
		t.Fatalf("parseRunArgs command = %q, want %q", cmd, "python train.py")
	}
}

func TestParseRunArgsSupportsSyncFlag(t *testing.T) {
	syncFirst, cmd, ok := parseRunArgs([]string{"--sync", "python train.py"})
	if !ok {
		t.Fatal("parseRunArgs returned ok=false")
	}
	if !syncFirst {
		t.Fatal("parseRunArgs syncFirst = false, want true")
	}
	if cmd != "python train.py" {
		t.Fatalf("parseRunArgs command = %q, want %q", cmd, "python train.py")
	}
}

func TestParseRunArgsRejectsMissingCommandAfterSyncFlag(t *testing.T) {
	if _, _, ok := parseRunArgs([]string{"--sync"}); ok {
		t.Fatal("parseRunArgs returned ok=true, want false")
	}
}

func TestParseBgRunArgsDefaultsToNoSync(t *testing.T) {
	syncFirst, job, cmd, ok := parseBgRunArgs([]string{"train", "python", "train.py"})
	if !ok {
		t.Fatal("parseBgRunArgs returned ok=false")
	}
	if syncFirst {
		t.Fatal("parseBgRunArgs syncFirst = true, want false")
	}
	if job != "train" {
		t.Fatalf("parseBgRunArgs job = %q, want %q", job, "train")
	}
	if cmd != "python train.py" {
		t.Fatalf("parseBgRunArgs command = %q, want %q", cmd, "python train.py")
	}
}

func TestParseBgRunArgsSupportsSyncFlag(t *testing.T) {
	syncFirst, job, cmd, ok := parseBgRunArgs([]string{"--sync", "train", "python train.py"})
	if !ok {
		t.Fatal("parseBgRunArgs returned ok=false")
	}
	if !syncFirst {
		t.Fatal("parseBgRunArgs syncFirst = false, want true")
	}
	if job != "train" {
		t.Fatalf("parseBgRunArgs job = %q, want %q", job, "train")
	}
	if cmd != "python train.py" {
		t.Fatalf("parseBgRunArgs command = %q, want %q", cmd, "python train.py")
	}
}

func TestParseBgRunArgsRejectsMissingJobOrCommand(t *testing.T) {
	cases := [][]string{
		{},
		{"--sync"},
		{"train"},
		{"--sync", "train"},
	}

	for _, args := range cases {
		if _, _, _, ok := parseBgRunArgs(args); ok {
			t.Fatalf("parseBgRunArgs(%v) returned ok=true, want false", args)
		}
	}
}

func mustWriteTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func chdirTemp(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustReadTestFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
