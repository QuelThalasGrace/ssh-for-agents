package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

func mustWriteTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
