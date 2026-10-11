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

// Only "run" and a complete, known "service <action>" are accepted; everything
// else must fail instead of silently starting the daemon in the foreground.
func TestParseSubcommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Subcommand
		wantErr bool
	}{
		{name: "no subcommand", args: nil},
		{name: "run", args: []string{"run"}, want: Subcommand{RunMode: true}},
		{name: "run with extra arg", args: []string{"run", "extra"}, wantErr: true},
		{name: "service install", args: []string{"service", "install"}, want: Subcommand{Action: "install"}},
		{name: "service uninstall", args: []string{"service", "uninstall"}, want: Subcommand{Action: "uninstall"}},
		{name: "service start", args: []string{"service", "start"}, want: Subcommand{Action: "start"}},
		{name: "service stop", args: []string{"service", "stop"}, want: Subcommand{Action: "stop"}},
		{name: "service status", args: []string{"service", "status"}, want: Subcommand{Action: "status"}},
		{name: "service without action", args: []string{"service"}, wantErr: true},
		{name: "service with unknown action", args: []string{"service", "reload"}, wantErr: true},
		{name: "service with extra arg", args: []string{"service", "install", "extra"}, wantErr: true},
		{name: "misspelled subcommand", args: []string{"servcie", "install"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubcommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSubcommand(%q) = %+v, want an error", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSubcommand(%q): %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("ParseSubcommand(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
