package projects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocExtension(t *testing.T) {
	cases := map[string]string{
		"notes.pdf": ".pdf",
		"README.MD": ".md",
		"spec.doc":  ".doc",
		"spec.docx": ".docx",
		"plain.txt": ".txt",
	}
	for name, want := range cases {
		got, err := docExtension(name)
		if err != nil {
			t.Fatalf("docExtension(%q) returned error: %v", name, err)
		}
		if got != want {
			t.Errorf("docExtension(%q) = %q, want %q", name, got, want)
		}
	}
	if _, err := docExtension("archive.zip"); err == nil {
		t.Error("docExtension(archive.zip) should fail")
	}
}

func TestStripDataURL(t *testing.T) {
	if got := stripDataURL("data:application/pdf;base64,AAAA"); got != "AAAA" {
		t.Errorf("stripDataURL with prefix = %q, want %q", got, "AAAA")
	}
	if got := stripDataURL("  AAAA  "); got != "AAAA" {
		t.Errorf("stripDataURL plain = %q, want %q", got, "AAAA")
	}
}

func TestChecksumIsStable(t *testing.T) {
	first := Checksum([]byte("hello"))
	second := Checksum([]byte("hello"))
	if first != second {
		t.Fatalf("checksum is not stable: %q != %q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("checksum length = %d, want 64", len(first))
	}
}

func TestSaveAndRemoveDocumentFile(t *testing.T) {
	projectID := "123e4567-e89b-12d3-a456-426614174000"
	path, err := SaveDocumentFile(projectID, Checksum([]byte("content")), ".txt", []byte("content"))
	if err != nil {
		t.Fatalf("SaveDocumentFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	if err := RemoveDocumentFile(path); err != nil {
		t.Fatalf("RemoveDocumentFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists after removal: %v", err)
	}

	// Clean up the project directory left behind by the test.
	dir, err := DocumentsDir(projectID)
	if err != nil {
		t.Fatalf("DocumentsDir: %v", err)
	}
	_ = os.RemoveAll(filepath.Dir(dir))
}

func TestRemoveDocumentFileRejectsOutsidePath(t *testing.T) {
	if err := RemoveDocumentFile("/etc/passwd"); err == nil {
		t.Fatal("expected RemoveDocumentFile to reject a path outside the app data directory")
	}
}
