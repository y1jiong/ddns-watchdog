package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kardianos/service"
)

// stopTimeout bounds how long Stop waits for the run function to return.
// systemd's default TimeoutStopSec is 90s and the Windows SCM gives up after
// about 30s, so waiting any longer only delays an unclean kill.
var stopTimeout = 10 * time.Second

// Svc wraps a kardianos service instance.
type Svc struct {
	inner service.Service
}

type program struct {
	runFunc func(context.Context)
	cancel  context.CancelFunc
	done    chan struct{}
}

func (p *program) Start(s service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)
		p.runFunc(ctx)
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// The run function is a long-lived loop (DNS check cycle / HTTP server), so it
	// only returns once ctx is cancelled. Wait for it, but not indefinitely:
	// blocking the OS service manager until its own timeout ends in SIGKILL.
	if p.cancel != nil {
		p.cancel()
	}
	if p.done == nil {
		return nil
	}

	select {
	case <-p.done:
	case <-time.After(stopTimeout):
		fmt.Fprintf(os.Stderr, "service did not stop within %v, exiting anyway\n", stopTimeout)
	}
	return nil
}

// New creates a service. extraArgs are appended to the binary command line when the service manager starts it.
// runFunc must return when ctx is cancelled; Stop cancels ctx and waits (up to stopTimeout) for that to happen.
func New(name, displayName, description string, extraArgs []string, runFunc func(context.Context)) (*Svc, error) {
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
		fmt.Fprintf(os.Stderr, "unknown service action: %s (want one of: %s)\n", action, strings.Join(Actions, ", "))
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
