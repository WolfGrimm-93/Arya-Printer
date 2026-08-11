package ssl

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultRenewThresholdDays matches run_setup()'s renew_threshold_days=30
// default in the Python service.
const defaultRenewThresholdDays = 30

func sslFilesExist(sslDir string) bool {
	if _, err := os.Stat(filepath.Join(sslDir, "server.crt")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(sslDir, "server.key")); err != nil {
		return false
	}
	return true
}

// RunSetup configures SSL for localhost using mkcert, following the same
// decision logic as run_setup() in the Python service:
//   - CA installed + cert files present and not near expiry -> nothing to do
//   - cert files present but at/near expiry                 -> remove CA, force full regeneration
//   - CA not installed                                       -> install it
//   - cert files missing                                     -> generate them
//
// installDir is where mkcert.exe lives; sslDir is where server.crt/
// server.key live. Returns 0 on success, 1 on failure — meant to be used
// directly as the process exit code for --setup-ssl.
func RunSetup(sslDir, installDir string) int {
	filesOK := sslFilesExist(sslDir)
	caOK := CAIsInstalled(installDir)
	needsRenewal := CertNeedsRenewal(sslDir, defaultRenewThresholdDays)
	days, _ := CertDaysRemaining(sslDir)

	if caOK && filesOK && !needsRenewal {
		if days != nil {
			fmt.Printf("SSL ya configurado — certificado válido por %d días más.\n", *days)
		}
		fmt.Println("Nada que hacer.")
		return 0
	}

	if filesOK && needsRenewal {
		switch {
		case days != nil && *days <= 0:
			fmt.Printf("Certificado SSL EXPIRADO hace %d días — renovando...\n", -*days)
		case days != nil:
			fmt.Printf("Certificado SSL expira en %d días — renovando...\n", *days)
		}
		if err := RemoveCA(installDir); err != nil {
			fmt.Printf("Advertencia: %v\n", err)
		}
		filesOK = false
		caOK = false
	}

	if !caOK {
		fmt.Println("Instalando CA de mkcert en los almacenes de confianza...")
		if err := InstallCA(installDir); err != nil {
			fmt.Printf("ERROR: No se pudo instalar la CA — probá ejecutando como Administrador (%v)\n", err)
			return 1
		}
	}

	if !filesOK {
		fmt.Println("Generando certificados SSL para localhost HTTPS...")
		if err := GenerateCerts(sslDir, installDir); err != nil {
			fmt.Printf("ERROR: No se pudieron generar los certificados (%v)\n", err)
			return 1
		}
	}

	fmt.Println()
	fmt.Println("Configuración SSL completa.")
	fmt.Println("El servicio ahora usará: https://localhost:58181")
	return 0
}

// RunRemove removes the mkcert CA and deletes server.crt/server.key from
// sslDir. Returns 0 on success, 1 if removing the CA failed — file deletion
// errors are logged but do not by themselves affect the exit code, matching
// the Python service's run_remove.
func RunRemove(sslDir, installDir string) int {
	fmt.Println("Eliminando CA de mkcert de los almacenes de confianza...")
	caErr := RemoveCA(installDir)
	if caErr != nil {
		fmt.Printf("Advertencia: %v\n", caErr)
	}

	for _, name := range []string{"server.crt", "server.key"} {
		p := filepath.Join(sslDir, name)
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		if rmErr := os.Remove(p); rmErr == nil {
			fmt.Printf("Eliminado: %s\n", p)
		} else {
			fmt.Printf("Advertencia: no se pudo eliminar %s: %v\n", p, rmErr)
		}
	}

	if caErr != nil {
		return 1
	}
	return 0
}
