package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.yaml")

	if err := writeFileAtomic(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("content = %q", string(data))
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", fi.Mode().Perm())
	}

	// Overwrite.
	if err := writeFileAtomic(path, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "world\n" {
		t.Errorf("after overwrite = %q", string(data))
	}

	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestWriteFileAtomicMissingDir(t *testing.T) {
	dir := t.TempDir()
	err := writeFileAtomic(filepath.Join(dir, "nope", "x"), []byte("y"), 0o644)
	if err == nil {
		t.Fatal("expected error writing into missing dir")
	}
}

func TestReportLifecycle(t *testing.T) {
	s := newTestStorage(t)

	if s.HasReport("PROJ", "1") {
		t.Error("HasReport should be false before writing")
	}
	if _, ok := s.ReadReport("PROJ", "1"); ok {
		t.Error("ReadReport ok should be false before writing")
	}

	if err := s.WriteReport("PROJ", "1", "## AC Results\n- [x] one\n- [ ] two\n"); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	if !s.HasReport("PROJ", "1") {
		t.Error("HasReport should be true after writing")
	}
	body, ok := s.ReadReport("PROJ", "1")
	if !ok {
		t.Fatal("ReadReport ok = false after write")
	}
	if !strings.Contains(body, "- [ ] two") {
		t.Errorf("report body = %q", body)
	}
}

func TestClearRemovesReports(t *testing.T) {
	s := newTestStorage(t)
	if err := s.WriteReport("PROJ", "1", "report"); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	removed, err := s.Clear("PROJ")
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if removed == 0 {
		t.Error("Clear should have removed report files")
	}
	if s.HasReport("PROJ", "1") {
		t.Error("report should be gone after clear")
	}
}