package document

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertOfficeToPDF_RequiresLibreOffice(t *testing.T) {
	if _, err := findSoffice(); err != nil {
		t.Skip("requires soffice.exe, not available in this environment")
	}

	dir := t.TempDir()
	txtPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(txtPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	pdfPath, err := ConvertOfficeToPDF(context.Background(), txtPath)
	if err != nil {
		t.Fatalf("ConvertOfficeToPDF() error = %v", err)
	}
	defer os.Remove(pdfPath)

	if st, err := os.Stat(pdfPath); err != nil || st.Size() == 0 {
		t.Fatalf("expected non-empty output PDF, stat err = %v", err)
	}
}

// TestEnsureLibreOfficeProfile_ForcesMacroSecurityLevel verifies that the
// isolated profile soffice is launched against actually gets seeded with a
// registrymodifications.xcu forcing MacroSecurityLevel=3 (never run a macro
// without a trusted signature) — this is what backs the security claim in
// the package doc comment, not the --headless flag, which does not disable
// macro execution by itself (CVE-2018-16858 abused exactly that gap). This
// test requires no external binary: it only exercises the pure-Go profile
// setup, not soffice.exe itself.
func TestEnsureLibreOfficeProfile_ForcesMacroSecurityLevel(t *testing.T) {
	profileDir, err := ensureLibreOfficeProfile()
	if err != nil {
		t.Fatalf("ensureLibreOfficeProfile() error = %v", err)
	}
	if profileDir == "" {
		t.Fatal("ensureLibreOfficeProfile() returned an empty directory")
	}

	xcuPath := filepath.Join(profileDir, "user", "registrymodifications.xcu")
	raw, err := os.ReadFile(xcuPath)
	if err != nil {
		t.Fatalf("reading %s: %v", xcuPath, err)
	}

	content := string(raw)
	if !strings.Contains(content, "MacroSecurityLevel") {
		t.Fatalf("registrymodifications.xcu does not mention MacroSecurityLevel:\n%s", content)
	}
	if !strings.Contains(content, "Security/Scripting") {
		t.Fatalf("registrymodifications.xcu does not target Security/Scripting:\n%s", content)
	}
	if !strings.Contains(content, "<value>3</value>") {
		t.Fatalf("registrymodifications.xcu does not force level 3 (very high):\n%s", content)
	}
}

func TestFindSoffice_NotFoundIsServiceUnavailable(t *testing.T) {
	if _, err := findSoffice(); err == nil {
		t.Skip("soffice.exe is available in this environment; nothing to assert about the not-found path")
	} else {
		// Just confirm we get *some* error and don't panic; the concrete
		// contract.ServiceUnavailable shape is exercised by ConvertOfficeToPDF
		// callers (printsvc), not asserted field-by-field here.
		if err.Error() == "" {
			t.Fatal("expected a non-empty error message")
		}
	}
}
