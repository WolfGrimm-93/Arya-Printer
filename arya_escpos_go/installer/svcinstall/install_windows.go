//go:build windows

// Package svcinstall installs and removes the AryaPrinter Windows Service
// (via golang.org/x/sys/windows/svc/mgr), replacing the Python service's
// SYSTEM-privileged Scheduled Task — audit finding: excessive privilege for
// a local printing agent that has no business running as SYSTEM.
//
// The service runs under a Virtual Service Account (NT SERVICE\AryaPrinter)
// instead of SYSTEM, LocalService or a manually managed account+password.
// Windows provisions and manages that account's credential automatically,
// scoped to this one service, with no interactive logon rights and no local
// admin membership by default. See:
// https://learn.microsoft.com/windows/win32/services/virtual-accounts
//
// # Integration with cmd/aryaprinter/main.go (Agente A)
//
// main.go owns the actual svc.Handler implementation that runs *inside* the
// service process once the SCM starts it (cmd/aryaprinter/service_windows.go).
// This package only talks to the SCM to register/unregister the service
// definition — main.go's flag handling should call it as a one-liner:
//
//	case "--install-service":
//	    exePath, err := os.Executable()
//	    ...
//	    if err := svcinstall.Install(exePath); err != nil { ... }
//
//	case "--uninstall-service":
//	    if err := svcinstall.Uninstall(); err != nil { ... }
//
// Both operations require an elevated (Administrator) process — mgr.Connect
// returns an error otherwise, which callers should surface verbatim (it
// already explains what to do).
package svcinstall

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceName is the internal Windows service name (the registry key under
// HKLM\SYSTEM\CurrentControlSet\Services). It MUST match the suffix of
// VirtualAccount below — that equality is what makes NT SERVICE\<ServiceName>
// a valid Virtual Service Account for this specific service. It also MUST
// match the name cmd/aryaprinter/service_windows.go (Agente A) passes to
// svc.Run — that file's own comment says as much ("Agent D's installer must
// register the service with this exact name so runAsService/isWindowsService
// line up with it"): if the SCM registration name and the svc.Run name
// differ, the service fails to start once launched by the SCM even though
// both halves compile fine independently.
const ServiceName = "AryaESCPOSService"

// ServiceDisplayName is shown in services.msc and `Get-Service`.
const ServiceDisplayName = "Arya Printer Service"

// ServiceDescription is shown in the services.msc "Description" column.
const ServiceDescription = "Servicio local de impresion ESC/POS, ESC/P y documentos para Arya Market. Atiende en 127.0.0.1:58181 (o el host/puerto configurado)."

// recoveryResetPeriod is how long the service must run without failing
// before the SCM resets its failure count back to 0 (windows.ChangeServiceConfig2's
// SERVICE_CONFIG_FAILURE_ACTIONS "reset period", in seconds). 24h: a service
// that fails once in a blue moon should not slowly exhaust its 3 configured
// actions and end up stuck with NoAction after a handful of unrelated
// failures spread across weeks.
const recoveryResetPeriod = uint32(24 * 60 * 60)

// VirtualAccount is the Virtual Service Account under which the service
// runs. Exported so installer/set_acl.ps1 (and any other installer step
// that needs to grant this identity file permissions, e.g. on logs/,
// configs/, ssl/, secrets/) can reference the exact same name instead of
// duplicating the "NT SERVICE\AryaPrinter" string.
const VirtualAccount = `NT SERVICE\` + ServiceName

// Install registers AryaPrinter as a Windows Service pointing at exePath,
// running under VirtualAccount with automatic start, then starts it
// immediately.
//
// If the service is already registered (e.g. re-running the installer to
// upgrade an existing install), Install stops and removes the previous
// registration first, then recreates it against the new exePath — this
// makes running the installer again over an existing install safe and
// idempotent, the same "don't duplicate, recreate cleanly" pattern used by
// installer/setup_firewall.ps1 for the firewall rule.
//
// Must run elevated (Administrator); mgr.Connect fails otherwise.
func Install(exePath string) error {
	if exePath == "" {
		return errors.New("svcinstall: exePath vacio")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("svcinstall: conectando al Service Control Manager (se requiere ejecutar como Administrador): %w", err)
	}
	defer m.Disconnect()

	if existing, err := m.OpenService(ServiceName); err == nil {
		if err := removeService(existing); err != nil {
			return fmt.Errorf("svcinstall: eliminando instalacion previa del servicio para reinstalar: %w", err)
		}
	}

	cfg := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  ServiceDisplayName,
		Description:  ServiceDescription,
		// Cuenta virtual de bajo privilegio (ver comentario de VirtualAccount
		// mas arriba). Password vacio: la SCM gestiona la credencial sola, no
		// hay contraseña que guardar, transmitir ni rotar manualmente.
		ServiceStartName: VirtualAccount,
		Password:         "",
	}

	s, err := m.CreateService(ServiceName, exePath, cfg)
	if err != nil {
		return fmt.Errorf("svcinstall: creando el servicio: %w", err)
	}
	defer s.Close()

	if err := configureRecoveryActions(s); err != nil {
		return fmt.Errorf("svcinstall: configurando el reinicio automatico tras un crash: %w", err)
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf(
			"svcinstall: el servicio se registro pero no pudo iniciarse "+
				"(revisar que %s tenga permisos sobre logs/, configs/, ssl/ y secrets/ "+
				"dentro del directorio de instalacion): %w",
			VirtualAccount, err,
		)
	}

	return nil
}

// configureRecoveryActions registers SERVICE_CONFIG_FAILURE_ACTIONS with the
// SCM so the service restarts itself after a crash instead of staying dead
// until someone notices and starts it by hand.
//
// This matters because Go turns a fatal runtime error — most relevantly an
// out-of-memory kill — into an unconditional process exit, not a panic:
// internal/middleware.Recover (Agente A) can only catch panics from within a
// request handler, it cannot intercept this. internal/apiserver's upload and
// image/barcode size limits (Agentes A/B) make that scenario far less likely
// to be reachable at all, but this is deliberately independent, coarser
// defense-in-depth: it also covers any *other* unanticipated fatal crash,
// not just the OOM one that prompted adding it.
//
// Windows only ever runs a recovery action when the process exits without
// telling the SCM it stopped on purpose (SERVICE_STOPPED) — an OOM kill (or
// any other fatal runtime crash) never reaches
// cmd/aryaprinter/service_windows.go's `changes <- svc.Status{State:
// svc.Stopped}` line, so it always counts as a failure here. A deliberate
// Stop/Shutdown request, or runServer returning an error that Execute
// reports as Stopped, does NOT trigger recovery — left as the default
// (SetRecoveryActionsOnNonCrashFailures is never set to true here), so a
// service that shuts itself down cleanly after a real, non-crash error is
// not endlessly respawned into failing the same way.
//
// Backoff: 1st failure -> restart after 5s, 2nd -> restart after 10s, 3rd
// and any further failure within recoveryResetPeriod -> NoAction (manual
// intervention required; repeatedly respawning into the same crash forever
// would just thrash instead of helping).
func configureRecoveryActions(s *mgr.Service) error {
	actions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.NoAction, Delay: 0},
	}
	return s.SetRecoveryActions(actions, recoveryResetPeriod)
}

// Uninstall stops (if running) and removes the AryaPrinter service
// registration. Returns nil if the service is not registered at all, so
// calling it on a machine where Install never ran — or twice — is safe.
//
// Must run elevated (Administrator); mgr.Connect fails otherwise.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("svcinstall: conectando al Service Control Manager (se requiere ejecutar como Administrador): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		// No registrado: nada que hacer.
		return nil
	}
	return removeService(s)
}

// removeService stops s (best-effort, tolerating "already stopped") and
// deletes its SCM registration. The caller owns s and is responsible for it
// being open; removeService always closes it before returning, regardless
// of outcome.
func removeService(s *mgr.Service) error {
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		if _, ctrlErr := s.Control(svc.Stop); ctrlErr != nil && !errors.Is(ctrlErr, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return fmt.Errorf("deteniendo el servicio: %w", ctrlErr)
		}
		if err := waitStopped(s, 10*time.Second); err != nil {
			return err
		}
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("eliminando el registro del servicio: %w", err)
	}
	return nil
}

// waitStopped polls s.Query until the service reports svc.Stopped or
// timeout elapses.
func waitStopped(s *mgr.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err != nil {
			return fmt.Errorf("consultando estado del servicio: %w", err)
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("el servicio no se detuvo dentro de %s", timeout)
}
