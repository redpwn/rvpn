//go:build windows

package service

// this code is largely sourced from: https://github.com/golang/sys/blob/master/windows/svc/example/service.go

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redpwn/rvpn/daemon"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName = "rvpnsvc"
	displayName = "RVPN Daemon Service"
	serviceDesc = "The rVPN daemon service is the underlying daemon responsible for managing the rVPN client wireguard interface. It exposes a RPC port for control from the client."
)

/* RVPNServiceMgr manages the rVPN service itself */

type RVPNServiceMgr struct{}

// Exists returns if the rVPN service exists
func (r *RVPNServiceMgr) Exists() (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return true, nil
	}

	// failed to open service, service does not exist
	return false, nil
}

// Install attempts to install the rVPN service
func (r *RVPNServiceMgr) Install() error {
	exepath, err := os.Executable()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", serviceName)
	}

	svcConfig := mgr.Config{
		DisplayName: displayName,
		Description: serviceDesc,
	}
	s, err = m.CreateService(serviceName, exepath, svcConfig, "daemon")
	if err != nil {
		return err
	}
	defer s.Close()

	err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func (r *RVPNServiceMgr) Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

/* RVPNService is the actual rVPN windows service */

var elog debug.Log

type rVPNService struct{}

// Execute is a handler which receives events from windows service manager
func (m *rVPNService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// start rVPN daemon, daemon is blocking so we start it in its own thread
	newDaemon := daemon.NewRVPNDaemon()
	go newDaemon.Start()

loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			elog.Info(1, "received shutdown command")

			newDaemon.Stop()
			break loop
		case svc.Pause:
			changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
		case svc.Continue:
			changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
		default:
			elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

// runService is a helper which runs windows services
func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &rVPNService{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

// EnsureServiceStarted ensures the rVPN daemon has been started
func EnsureServiceStarted() error {
	rvpnServiceMgr := RVPNServiceMgr{}
	exists, err := rvpnServiceMgr.Exists()
	if err != nil {
		return err
	}

	if !exists {
		// we know the service does not exist, create it
		log.Println("rVPN service does not exist, installing service...")

		err := rvpnServiceMgr.Install()
		if err != nil {
			return err
		}
	}

	// the service exists at this point, we should start the service
	err = rvpnServiceMgr.Start()
	if err != nil {
		// this will return an error if there is already an instance started, catch this and silently pass
		if err.Error() == "could not start service: An instance of the service is already running." {
			return nil
		} else {
			// this is a legitimate error
			return err
		}
	}

	return nil
}

func StartRVPNDaemon() {
	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to detect if windows service: %v", err)
	}

	if !inService {
		// if not service, ensure rvpn service is running
		err := EnsureServiceStarted()
		if err != nil {
			log.Fatalf("failed to ensure service started: %v", err)
		}

		log.Println("rVPN service started")
	} else {
		// if service, run rVPN daemon
		runService(serviceName, false)
	}
}
