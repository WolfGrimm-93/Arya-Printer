package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSelfSignedCert writes a PEM-encoded self-signed certificate to
// <dir>/server.crt with NotAfter = now + validFor, for testing
// CertDaysRemaining/CertNeedsRenewal without depending on mkcert.exe.
func writeSelfSignedCert(t *testing.T, dir string, validFor time.Duration) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	notBefore := time.Now().Add(-time.Hour)
	notAfter := notBefore.Add(validFor + time.Hour)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath := filepath.Join(dir, "server.crt")
	f, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create %s: %v", certPath, err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("pem encode: %v", err)
	}
}

func TestCertDaysRemaining_NoFile(t *testing.T) {
	dir := t.TempDir()
	days, err := CertDaysRemaining(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days != nil {
		t.Fatalf("days = %v, want nil (no cert file)", *days)
	}
}

func TestCertDaysRemaining_ValidCert(t *testing.T) {
	dir := t.TempDir()
	writeSelfSignedCert(t, dir, 60*24*time.Hour) // ~60 days remaining

	days, err := CertDaysRemaining(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days == nil {
		t.Fatal("days = nil, want a value")
	}
	if *days < 58 || *days > 61 {
		t.Fatalf("days = %d, want ~60", *days)
	}
}

func TestCertDaysRemaining_ExpiredCert(t *testing.T) {
	dir := t.TempDir()
	// NotAfter in the past: use a negative validFor so notAfter < now.
	writeSelfSignedCert(t, dir, -48*time.Hour)

	days, err := CertDaysRemaining(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days == nil {
		t.Fatal("days = nil, want a value")
	}
	if *days >= 0 {
		t.Fatalf("days = %d, want negative (expired)", *days)
	}
}

func TestCertDaysRemaining_UnparsableFallsBackToFileAge(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	if err := os.WriteFile(certPath, []byte("not a real certificate"), 0o644); err != nil {
		t.Fatalf("write bad cert: %v", err)
	}

	days, err := CertDaysRemaining(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days == nil {
		t.Fatal("days = nil, want fallback estimate based on file age")
	}
	// File was just created, so age-based estimate should be close to the
	// full 730-day mkcert default validity.
	if *days < 725 || *days > 730 {
		t.Fatalf("days = %d, want ~730 (fresh file, fallback estimate)", *days)
	}
}

func TestCertNeedsRenewal(t *testing.T) {
	dirNoFile := t.TempDir()
	if !CertNeedsRenewal(dirNoFile, 30) {
		t.Error("CertNeedsRenewal() = false for missing cert, want true")
	}

	dirFresh := t.TempDir()
	writeSelfSignedCert(t, dirFresh, 365*24*time.Hour)
	if CertNeedsRenewal(dirFresh, 30) {
		t.Error("CertNeedsRenewal() = true for a cert valid ~365 days with a 30-day threshold, want false")
	}

	dirExpiringSoon := t.TempDir()
	writeSelfSignedCert(t, dirExpiringSoon, 10*24*time.Hour)
	if !CertNeedsRenewal(dirExpiringSoon, 30) {
		t.Error("CertNeedsRenewal() = false for a cert expiring in ~10 days with a 30-day threshold, want true")
	}
}
