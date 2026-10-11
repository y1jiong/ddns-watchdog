package service

import (
	"context"
	"testing"
	"time"
)

// Stop must cancel the run function's context and return once it does. Blocking
// indefinitely makes systemd wait out TimeoutStopSec (SIGKILL) and makes the
// Windows SCM report that the service did not respond.
func TestStopCancelsRunFunc(t *testing.T) {
	started := make(chan struct{})
	returned := make(chan struct{})

	p := &program{runFunc: func(ctx context.Context) {
		close(started)
		defer close(returned)
		<-ctx.Done()
	}}

	if err := p.Start(nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started

	done := make(chan error, 1)
	go func() { done <- p.Stop(nil) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop blocked; it must return after cancelling the run function's context")
	}

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Stop returned before the run function returned")
	}
}

// A run function that ignores its context must not block Stop forever.
func TestStopIsBounded(t *testing.T) {
	orig := stopTimeout
	stopTimeout = 50 * time.Millisecond
	defer func() { stopTimeout = orig }()

	blocked := make(chan struct{})
	p := &program{runFunc: func(ctx context.Context) { <-blocked }}
	if err := p.Start(nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer close(blocked)

	start := time.Now()
	if err := p.Stop(nil); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Stop waited %v, want it bounded by stopTimeout", elapsed)
	}
}

// Stop may be called without a preceding Start (e.g. uninstall before install).
func TestStopWithoutStart(t *testing.T) {
	p := &program{}
	if err := p.Stop(nil); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
