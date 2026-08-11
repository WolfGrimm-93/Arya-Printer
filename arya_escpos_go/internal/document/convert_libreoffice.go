package document

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aryaescpos/internal/contract"
)

// libreOfficeMu serializes every soffice.exe --headless invocation. Two
// concurrent headless instances race over the same LibreOffice user profile
// lock and can fail or hang; for v1 it is far simpler (and safe enough for a
// local single-user print agent) to serialize conversions than to give each
// invocation its own profile directory.
var libreOfficeMu sync.Mutex

// libreOfficeConvertTimeout bounds a single soffice.exe conversion. If it is
// exceeded the process is killed via context cancellation.
const libreOfficeConvertTimeout = 60 * time.Second

// macroSecurityRegistryXCU is a minimal LibreOffice user-config fragment
// that forces Security/Scripting/MacroSecurityLevel to 3 ("Very High": never
// execute a macro unless it comes from a trusted source). This profile never
// configures any trusted source/certificate, so in practice this blocks
// every macro. --headless does NOT do this by itself — soffice --convert-to
// on an attacker-controlled document is the exact vector behind
// CVE-2018-16858 — so this file is what actually backs the security claim
// in the package doc comment, not the --headless flag.
const macroSecurityRegistryXCU = `<?xml version="1.0" encoding="UTF-8"?>
<oor:items xmlns:oor="http://openoffice.org/2001/registry" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
 <item oor:path="/org.openoffice.Office.Common/Security/Scripting"><prop oor:name="MacroSecurityLevel" oor:op="fuse"><value>3</value></prop></item>
</oor:items>
`

// libreOfficeProfileDirName is the isolated LibreOffice user profile
// directory (under os.TempDir()) used for every headless conversion, passed
// via -env:UserInstallation so soffice never touches a real user profile
// and always launches with the macro security policy above already in
// place.
const libreOfficeProfileDirName = "aryaescpos-lo-profile"

var (
	libreOfficeProfileOnce sync.Once
	libreOfficeProfileDir  string
	libreOfficeProfileErr  error
)

// ensureLibreOfficeProfile creates (once per process; safe to call
// repeatedly) the isolated profile directory and seeds its
// registrymodifications.xcu with the macro security policy above, before
// soffice ever runs. It returns the profile's root directory, to be passed
// as -env:UserInstallation=file:///<dir>.
func ensureLibreOfficeProfile() (string, error) {
	libreOfficeProfileOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), libreOfficeProfileDirName)
		userDir := filepath.Join(dir, "user")
		if err := os.MkdirAll(userDir, 0o700); err != nil {
			libreOfficeProfileErr = fmt.Errorf("creating LibreOffice profile dir: %w", err)
			return
		}
		xcuPath := filepath.Join(userDir, "registrymodifications.xcu")
		if err := os.WriteFile(xcuPath, []byte(macroSecurityRegistryXCU), 0o600); err != nil {
			libreOfficeProfileErr = fmt.Errorf("seeding LibreOffice macro security policy: %w", err)
			return
		}
		libreOfficeProfileDir = dir
	})
	return libreOfficeProfileDir, libreOfficeProfileErr
}

var sofficeCandidatePaths = []string{
	`C:\Program Files\LibreOffice\program\soffice.exe`,
	`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
}

func findSoffice() (string, error) {
	for _, p := range sofficeCandidatePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("soffice.exe"); err == nil {
		return p, nil
	}
	return "", contract.ServiceUnavailable(
		"libreoffice_not_found",
		"LibreOffice no está instalado",
		"No se encontró soffice.exe en las rutas esperadas (C:\\Program Files\\LibreOffice\\program\\soffice.exe, "+
			"C:\\Program Files (x86)\\LibreOffice\\program\\soffice.exe) ni en PATH. Instalá LibreOffice "+
			"(https://www.libreoffice.org/download/) para poder convertir documentos Word/Excel a PDF.",
	)
}

// ConvertOfficeToPDF converts a Word (.doc/.docx) or Excel (.xls/.xlsx)
// document at inputPath to PDF using LibreOffice headless. This is the
// direct replacement for the Python service's COM automation of a real
// Word/Excel instance — see the package doc comment for why that was a
// critical security finding. The returned path is a temp file owned by the
// caller, who must remove it once done (e.g. after printing).
func ConvertOfficeToPDF(ctx context.Context, inputPath string) (string, error) {
	soffice, err := findSoffice()
	if err != nil {
		return "", err
	}

	profileDir, err := ensureLibreOfficeProfile()
	if err != nil {
		return "", contract.BadGateway("libreoffice_profile_failed", "No se pudo preparar el perfil aislado de LibreOffice con la política de seguridad de macros", err.Error())
	}
	userInstallationArg := "-env:UserInstallation=file:///" + filepath.ToSlash(profileDir)

	outDir, err := os.MkdirTemp("", "aryaescpos-lo-*")
	if err != nil {
		return "", contract.BadGateway("libreoffice_conversion_failed", "No se pudo crear el directorio temporal de conversión", err.Error())
	}
	defer os.RemoveAll(outDir)

	libreOfficeMu.Lock()
	defer libreOfficeMu.Unlock()

	cctx, cancel := context.WithTimeout(ctx, libreOfficeConvertTimeout)
	defer cancel()

	// --norestore avoids soffice trying to reopen documents from a crashed
	// previous session (which can pop a recovery dialog and hang headless
	// mode indefinitely). userInstallationArg is what actually forces the
	// macro security policy (see ensureLibreOfficeProfile) — --headless does
	// not do this on its own.
	cmd := exec.CommandContext(cctx, soffice, "--headless", "--norestore", userInstallationArg, "--convert-to", "pdf", "--outdir", outDir, inputPath)
	out, runErr := cmd.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return "", contract.BadGateway("libreoffice_conversion_timeout", "La conversión con LibreOffice superó el tiempo límite", strings.TrimSpace(string(out)))
	}
	if runErr != nil {
		return "", contract.BadGateway("libreoffice_conversion_failed", "LibreOffice no pudo convertir el documento a PDF", strings.TrimSpace(string(out))+" | "+runErr.Error())
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	convertedPath := filepath.Join(outDir, base+".pdf")
	if st, statErr := os.Stat(convertedPath); statErr != nil || st.IsDir() {
		return "", contract.BadGateway("libreoffice_conversion_failed", "LibreOffice no generó el PDF de salida esperado", fmt.Sprintf("esperado en %s; salida: %s", convertedPath, strings.TrimSpace(string(out))))
	}

	// Move the result out of outDir (which gets removed by the defer above)
	// into its own temp file so the caller has a single, stable path to own.
	finalPath, err := newTempPDFPath("aryaescpos-office")
	if err != nil {
		return "", contract.BadGateway("libreoffice_conversion_failed", "No se pudo reservar el archivo PDF de salida", err.Error())
	}
	if err := moveFile(convertedPath, finalPath); err != nil {
		return "", contract.BadGateway("libreoffice_conversion_failed", "No se pudo mover el PDF convertido", err.Error())
	}
	return finalPath, nil
}

// moveFile renames src to dst, falling back to copy+remove if they are on
// different volumes (os.Rename fails with an error in that case on Windows).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
