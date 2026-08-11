// Command aryaprinter is the Arya ESCPOS Go agent: a local Windows service
// that lets the Arya Market web app print to thermal/dot-matrix/document
// printers it cannot reach directly from the browser.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"aryaescpos/installer/svcinstall"
	"aryaescpos/internal/apiserver"
	"aryaescpos/internal/auth"
	"aryaescpos/internal/config"
	"aryaescpos/internal/history"
	"aryaescpos/internal/hwadapter"
	"aryaescpos/internal/logging"
	"aryaescpos/internal/middleware"
	"aryaescpos/internal/printsvc"
	"aryaescpos/internal/ssl"
	"aryaescpos/internal/winspool"
)

const defaultConfigPath = "configs/settings.yaml"

// installDir returns the directory containing the running executable — the
// same convention internal/document and internal/hwadapter already use to
// locate vendored tools (libusb-1.0.dll, PDFtoPrinter.exe) next to the
// binary, and the layout installer/installer.iss produces under {app}.
// Falls back to the working directory if the executable path can't be
// resolved, matching a plain `go run` from the project root in dev.
func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// sslDir returns <install dir>/ssl, matching installer.iss's {app}\ssl and
// the Python service's SSL_DIR convention (mkcert.exe writes server.crt/
// server.key there; api server checks for their existence at startup).
func sslDir() string {
	return filepath.Join(installDir(), "ssl")
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func main() {
	configPath := flag.String("config", defaultConfigPath, "path to settings.yaml")
	setupSSL := flag.Bool("setup-ssl", false, "generate and install a local dev TLS certificate")
	removeSSL := flag.Bool("remove-ssl", false, "remove the previously installed local dev TLS certificate")
	showAPIKey := flag.Bool("show-api-key", false, "print the current API key and exit")
	installService := flag.Bool("install-service", false, "install this binary as a Windows service")
	uninstallService := flag.Bool("uninstall-service", false, "uninstall the Windows service")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aryaprinter: loading config: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *showAPIKey:
		key, err := auth.Show(cfg.Security.APIKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "aryaprinter: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(key)
		return

	case *setupSSL:
		os.Exit(ssl.RunSetup(sslDir(), installDir()))

	case *removeSSL:
		os.Exit(ssl.RunRemove(sslDir(), installDir()))

	case *installService:
		exePath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "aryaprinter: resolving executable path: %v\n", err)
			os.Exit(1)
		}
		if err := svcinstall.Install(exePath); err != nil {
			fmt.Fprintf(os.Stderr, "aryaprinter: installing service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service installed and started.")
		return

	case *uninstallService:
		if err := svcinstall.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "aryaprinter: uninstalling service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service uninstalled.")
		return
	}

	logger, cleanup := logging.New(cfg.Logging)
	defer cleanup()

	asService, svcErr := isWindowsService()
	if svcErr == nil && asService {
		if err := runAsService(logger, cfg); err != nil {
			logger.Error("windows service exited with error", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runServer(ctx, cfg, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

// runServer builds the full handler chain — apiserver's routes wrapped by
// internal/middleware (recovery, request logging, auth, CORS, body limit)
// — and serves it on cfg.Server.Host:Port until ctx is cancelled, then
// shuts down gracefully. Shared by the interactive and Windows-service
// startup paths.
func runServer(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	apiKey, err := auth.LoadOrCreate(cfg.Security.APIKeyPath)
	if err != nil {
		return fmt.Errorf("main: loading api key: %w", err)
	}

	wq := winspool.NewQueue()
	factory := hwadapter.NewFactory(cfg.NetworkScan)

	deps := apiserver.Deps{
		Scanner:       printsvc.NewDeviceScanner(wq, cfg.NetworkScan),
		PrintQueue:    wq,
		Ticket:        printsvc.NewTicketPrinter(factory, wq),
		Matrix:        printsvc.NewMatrixPrinter(factory, wq),
		Document:      printsvc.NewDocumentPrinter(wq),
		History:       history.NewStore(),
		ConfigView:    sanitizedConfigView(cfg),
		MaxImageBytes: cfg.Security.MaxImageBytes,
	}

	handler := buildHandler(deps, cfg, apiKey, logger)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: handler}

	// Matches the Python service's startup check: if a previously generated
	// cert/key pair is present under <install dir>/ssl (created by
	// --setup-ssl), serve HTTPS; otherwise plain HTTP. Renewal is only
	// handled by --setup-ssl itself, not automatically at every startup —
	// same division of responsibility as ssl_setup.py.
	certPath := filepath.Join(sslDir(), "server.crt")
	keyPath := filepath.Join(sslDir(), "server.key")
	useTLS := fileExists(certPath) && fileExists(keyPath)

	errCh := make(chan error, 1)
	go func() {
		if useTLS {
			logger.Info("listening", "addr", addr, "protocol", "https")
			errCh <- srv.ListenAndServeTLS(certPath, keyPath)
		} else {
			logger.Info("listening", "addr", addr, "protocol", "http")
			errCh <- srv.ListenAndServe()
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// buildHandler assembles apiserver's routes wrapped in the full
// internal/middleware chain. Extracted out of runServer purely so tests can
// exercise the real, fully-wired handler (auth + CORS + the routes
// themselves) via httptest without opening a TCP listener — in particular
// the regression test in main_test.go that asserts GET /api/v1/config never
// leaks api_key_path over the wire, not just in sanitizedConfigView's
// return value in isolation.
func buildHandler(deps apiserver.Deps, cfg config.Config, apiKey string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	apiserver.New(deps, mux)

	// Order matters: each middleware.X(handler) call wraps the previous
	// handler, so execution order is the REVERSE of application order here
	// (outermost = last applied = Recover). CORS must wrap Auth (be applied
	// after it) so an OPTIONS preflight — which browsers send without the
	// X-API-Key header for any non-simple request like our JSON POSTs —
	// reaches middleware.CORS's short-circuit (204 + CORS headers) before
	// middleware.Auth would otherwise reject it with 401 and no CORS
	// headers, silently breaking every cross-origin call from Arya Market
	// when auth_enabled is true (the default).
	var handler http.Handler = mux
	handler = middleware.BodyLimit(cfg.Security.MaxUploadBytes)(handler)
	handler = middleware.Auth(cfg.Security.AuthEnabled, func(candidate string) bool {
		return auth.Valid(apiKey, candidate)
	})(handler)
	handler = middleware.CORS()(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.Recover(logger)(handler)
	return handler
}

// sanitizedConfigView builds the JSON snapshot GET /api/v1/config returns:
// every section of cfg, field by field with the same snake_case names as
// configs/settings.yaml, EXCEPT Security.APIKeyPath — a filesystem path to
// the key file, not the key material itself, but still not something to
// hand back over HTTP. Built as explicit maps rather than json-marshaling
// the config.* structs directly because those only carry yaml tags (Phase
// 0, frozen), so a direct marshal would emit Go's default capitalized
// field names instead of the snake_case the Arya Market frontend expects.
func sanitizedConfigView(cfg config.Config) any {
	return map[string]any{
		"server": map[string]any{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		},
		"logging": map[string]any{
			"level":          cfg.Logging.Level,
			"log_dir":        cfg.Logging.LogDir,
			"retention_days": cfg.Logging.RetentionDays,
		},
		"discovery": map[string]any{
			"auto_scan_interval": cfg.Discovery.AutoScanInterval,
			"usb_enabled":        cfg.Discovery.USBEnabled,
			"network_enabled":    cfg.Discovery.NetworkEnabled,
			"serial_enabled":     cfg.Discovery.SerialEnabled,
		},
		"devices": map[string]any{
			"auto_reconnect":     cfg.Devices.AutoReconnect,
			"connection_timeout": cfg.Devices.ConnectionTimeout,
			"retry_attempts":     cfg.Devices.RetryAttempts,
		},
		"printers": map[string]any{
			"paper_width": cfg.Printers.PaperWidth,
		},
		"network_scan": map[string]any{
			"enabled": cfg.NetworkScan.Enabled,
			"subnets": cfg.NetworkScan.Subnets,
			"ports":   cfg.NetworkScan.Ports,
			"timeout": cfg.NetworkScan.Timeout,
		},
		"security": map[string]any{
			"auth_enabled":     cfg.Security.AuthEnabled,
			"max_upload_bytes": cfg.Security.MaxUploadBytes,
			"max_image_bytes":  cfg.Security.MaxImageBytes,
			// api_key_path intentionally omitted.
		},
	}
}
