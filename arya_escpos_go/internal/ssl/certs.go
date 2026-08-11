package ssl

import (
	"crypto/x509"
	"encoding/pem"
	"math"
	"os"
	"path/filepath"
	"time"
)

// mkcertDefaultValidityDays is the certificate lifetime mkcert generates by
// default, used as the fallback estimate below.
const mkcertDefaultValidityDays = 730

// CertDaysRemaining returns the number of days until sslDir/server.crt
// expires. It returns (nil, nil) if the file does not exist. If the file
// exists but cannot be parsed as an X.509 certificate, it falls back to
// estimating remaining days from the file's modification time assuming a
// 730-day (mkcert default) validity — matching cert_days_remaining()'s
// fallback in the Python service, which does the same using the
// `cryptography` package's failure path.
func CertDaysRemaining(sslDir string) (*int, error) {
	certPath := filepath.Join(sslDir, "server.crt")
	info, err := os.Stat(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if raw, readErr := os.ReadFile(certPath); readErr == nil {
		if block, _ := pem.Decode(raw); block != nil {
			if cert, parseErr := x509.ParseCertificate(block.Bytes); parseErr == nil {
				remaining := int(math.Floor(time.Until(cert.NotAfter).Hours() / 24))
				return &remaining, nil
			}
		}
	}

	ageDays := time.Since(info.ModTime()).Hours() / 24
	remaining := int(mkcertDefaultValidityDays - ageDays)
	if remaining < 0 {
		remaining = 0
	}
	return &remaining, nil
}

// CertNeedsRenewal returns true if sslDir has no certificate at all, or if
// it expires within thresholdDays (including already expired).
func CertNeedsRenewal(sslDir string, thresholdDays int) bool {
	days, err := CertDaysRemaining(sslDir)
	if err != nil || days == nil {
		return true
	}
	return *days <= thresholdDays
}
