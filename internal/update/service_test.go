package update

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerPathsKeepExtension(t *testing.T) {
	tmp, final := installerPaths("/tmp/downloads", "rag-studio_0.1.1_amd64.deb")

	if !strings.HasSuffix(tmp, ".deb.part") {
		t.Fatalf("temp path lost the extension: %s", tmp)
	}
	if !strings.HasSuffix(final, ".deb") {
		t.Fatalf("final path must stay a .deb: %s", final)
	}
	if filepath.Ext(final) != ".deb" {
		t.Fatalf("filepath.Ext = %q, want .deb", filepath.Ext(final))
	}
}
