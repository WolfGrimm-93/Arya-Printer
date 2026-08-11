//go:build windows

// Windows Service Control Manager integration. This file only provides the
// svc.Handler boilerplate and the run/detect helpers main.go calls —
// registering/unregistering the service itself (the body of
// --install-service/--uninstall-service) lives in installer/svcinstall and
// is wired from main.go.
package main

import (
	"context"
	"log/slog"

	"golang.org/x/sys/windows/svc"

	"aryaescpos/internal/config"
)

// serviceName is the Windows service name this binary runs under once
// installed. Agent D's installer must register the service with this exact
// name so runAsService/isWindowsService line up with it.
const serviceName = "AryaESCPOSService"

// aryaService adapts runServer (the same startup path used when running
// interactively from a console) to golang.org/x/sys/windows/svc.Handler.
type aryaService struct {
	cfg    config.Config
	logger *slog.Logger
}

// Execute implements svc.Handler. It starts runServer in the background,
// reports Running to the Service Control Manager, and cancels the server's
// context on Stop/Shutdown, waiting for it to shut down gracefully before
// reporting Stopped.
func (s *aryaService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer(ctx, s.cfg, s.logger)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				s.logger.Error("server exited unexpectedly", "error", err)
			}
			changes <- svc.Status{State: svc.Stopped}
			return false, 0

		case req := <-r:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				// Wait for runServer's graceful shutdown to actually
				// finish before telling the SCM we're stopped.
				if err := <-errCh; err != nil {
					s.logger.Error("server exited unexpectedly during shutdown", "error", err)
				}
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}

// runAsService blocks running this process as the AryaESCPOSService
// Windows service, called from main() once isWindowsService() reports this
// process was launched by the Service Control Manager rather than
// interactively.
func runAsService(logger *slog.Logger, cfg config.Config) error {
	return svc.Run(serviceName, &aryaService{cfg: cfg, logger: logger})
}

// isWindowsService reports whether this process was launched by the
// Windows Service Control Manager.
func isWindowsService() (bool, error) {
	return svc.IsWindowsService()
}
