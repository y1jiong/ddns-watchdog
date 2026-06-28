package service

import (
	"fmt"
	"os"

	"github.com/kardianos/service"
)

// Svc wraps a kardianos service instance.
type Svc struct {
	inner service.Service
}

type program struct {
	exit    chan struct{}
	runFunc func()
}

func (p *program) Start(s service.Service) error {
	p.exit = make(chan struct{})
	go func() {
		defer close(p.exit)
		p.runFunc()
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// block until runFunc returns so the OS service manager gets an accurate stopped signal
	if p.exit != nil {
		<-p.exit
	}
	return nil
}

// New creates a service. extraArgs are appended to the binary command line when the service manager starts it.
func New(name, displayName, description string, extraArgs []string, runFunc func()) (*Svc, error) {
	cfg := &service.Config{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		// service manager invokes: binary run <extraArgs>; main() detects "run" and calls svc.Run()
		Arguments: append([]string{"run"}, extraArgs...),
	}
	s, err := service.New(&program{runFunc: runFunc}, cfg)
	if err != nil {
		return nil, err
	}
	return &Svc{inner: s}, nil
}

// RunCommand executes a service management action: install, uninstall, start, stop, status.
func RunCommand(s *Svc, action string) {
	var svcErr error
	switch action {
	case "install":
		svcErr = s.inner.Install()
	case "uninstall":
		_ = s.inner.Stop()
		svcErr = s.inner.Uninstall()
	case "start":
		svcErr = s.inner.Start()
	case "stop":
		svcErr = s.inner.Stop()
	case "status":
		status, err := s.inner.Status()
		if err != nil {
			svcErr = err
		} else {
			switch status {
			case service.StatusRunning:
				fmt.Println("service is running")
			case service.StatusStopped:
				fmt.Println("service is stopped")
			default:
				fmt.Println("service is not installed or status unknown")
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown service action: %s\n", action)
		os.Exit(1)
	}

	if svcErr != nil {
		fmt.Fprintf(os.Stderr, "service %s failed: %v\n", action, svcErr)
		os.Exit(1)
	}
	if action != "status" {
		fmt.Printf("service %s succeeded\n", action)
	}
}

// Run starts the service in daemon mode (called by the OS service manager).
func Run(s *Svc) {
	if err := s.inner.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "service run failed: %v\n", err)
		os.Exit(1)
	}
}
