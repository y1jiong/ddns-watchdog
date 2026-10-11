package service

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// Actions lists the service management actions accepted by RunCommand.
var Actions = []string{"install", "uninstall", "start", "stop", "status"}

// Subcommand is the positional subcommand on the command line, if any.
type Subcommand struct {
	// RunMode is set when the binary was invoked as "run", i.e. by the OS service manager.
	RunMode bool
	// Action is the requested service management action: install, uninstall, start, stop or status.
	Action string
}

// ParseSubcommand interprets the positional arguments that pflag left over in
// flag.Args(). No argument at all is the plain CLI flow. An unknown, incomplete or
// misspelled subcommand is an error so that it cannot silently launch the daemon
// in the foreground.
func ParseSubcommand(args []string) (Subcommand, error) {
	switch {
	case len(args) == 0:
		return Subcommand{}, nil

	case args[0] == "run":
		if len(args) > 1 {
			return Subcommand{}, unexpectedArg(args[1])
		}
		return Subcommand{RunMode: true}, nil

	case args[0] == "service":
		if len(args) < 2 {
			return Subcommand{}, fmt.Errorf("service requires an action (want one of: %s)", strings.Join(Actions, ", "))
		}
		if len(args) > 2 {
			return Subcommand{}, unexpectedArg(args[2])
		}
		if !slices.Contains(Actions, args[1]) {
			return Subcommand{}, fmt.Errorf("unknown service action %q (want one of: %s)", args[1], strings.Join(Actions, ", "))
		}
		return Subcommand{Action: args[1]}, nil

	default:
		return Subcommand{}, unexpectedArg(args[0])
	}
}

func unexpectedArg(arg string) error {
	return fmt.Errorf("unexpected argument %q", arg)
}

// PrintUsage writes the command line synopsis. pflag's own usage only lists the
// flags, so the positional subcommands have to be spelled out here.
func PrintUsage(w io.Writer, cmd string, printFlags func()) {
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s [flags]\n", cmd)
	fmt.Fprintf(w, "  %s run [flags]\n", cmd)
	fmt.Fprintf(w, "  %s service <action> [flags]\n", cmd)
	fmt.Fprintf(w, "\nSubcommands:\n")
	fmt.Fprintf(w, "  run                 run in the foreground, the way the OS service manager starts it\n")
	fmt.Fprintf(w, "  service <action>    manage the OS service: %s\n", strings.Join(Actions, ", "))
	fmt.Fprintf(w, "                      (-I and -U are shortcuts for install and uninstall)\n")
	fmt.Fprintf(w, "\nFlags:\n")
	printFlags()
}
