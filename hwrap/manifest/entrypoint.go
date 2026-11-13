package manifest

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

// Start performs a CLI parsing step to select the command
// we want to run and to dynamically extract additional flags
// related to the manifest, then runs the selected function
// on the manifest.
func Start(manifest *Manifest, name string, printUsage func()) {
	fi, _ := os.Stderr.Stat()
	tty := fi.Mode()&os.ModeCharDevice != 0
	slog.SetDefault(slog.New(NewColorHandler(tty && os.Getenv("NO_COLOR") == "")))

	// Add harness name to logs
	slog.SetDefault(slog.Default().WithGroup(name))

	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	var err error
	status := -1

	switch args[0] {
	case "setup":
		_ = parseInputFlags(manifest, []string{}, printUsage)
		status, err = Setup(manifest)
		break
	case "run":
		var report Report
		inputs := parseInputFlags(manifest, args[1:], printUsage)
		report, err = Run(manifest, inputs)
		status = report.Status

		// Print report to stdout
		json, _ := json.MarshalIndent(report, "", "  ")
		fmt.Printf("%s\n", json)
		break
	case "cleanup":
		_ = parseInputFlags(manifest, []string{}, printUsage)
		status, err = Cleanup(manifest)
		break
	case "output":
		_ = parseInputFlags(manifest, []string{}, printUsage)
		Output(manifest)
		status = 0
		break
	default:
		status = 1
		err = fmt.Errorf("Command does not exist: %s", args[0])
	}

	if err != nil {
		slog.Error(err.Error())
	}

	os.Exit(status)
}

// parseInputFlags sets up argument parsing based on the inputs
// specified on a manifest definition.
func parseInputFlags(m *Manifest, args []string, printUsage func()) map[string]FlagValue {
	fs := flag.NewFlagSet("main", flag.ExitOnError)
	help := fs.Bool("help", false, "print help")
	values := make(map[string]FlagValue, len(m.Inputs))
	for _, in := range m.Inputs {
		values[in.Name] = NewFlagValue(fs.String(in.Name, in.Default, in.Description))
	}

	if err := fs.Parse(args); err != nil {
		slog.Error(fmt.Sprintf("Failed to parse command line flags"))
		printUsage()
		os.Exit(1)
	}

	if *help {
		printUsage()
		os.Exit(1)
	}

	return values
}

// Color handling logger
//
// Shows different colors depending on the level. Useful for
// differentiating the output of this program from the output
// of the commands called from here.

type colorHandler struct {
	w       *os.File
	color   bool
	harness string
}

func NewColorHandler(color bool) colorHandler {
	return colorHandler{
		w:       os.Stderr,
		color:   color,
		harness: "default",
	}
}

func (h colorHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h colorHandler) Handle(_ context.Context, r slog.Record) error {
	color := map[slog.Level]string{
		slog.LevelDebug: "\x1b[1;36m", // bold cyan
		slog.LevelInfo:  "\x1b[1;32m", // bold green
		slog.LevelWarn:  "\x1b[1;33m", // bold yellow
		slog.LevelError: "\x1b[1;31m", // bold red
	}[r.Level]

	if !h.color {
		color = ""
	}

	fmt.Fprintf(h.w, "%s[%s] %s", color, h.harness, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
		return true
	})
	fmt.Fprintf(h.w, "\x1b[0m")
	fmt.Fprintln(h.w)
	return nil
}

func (h colorHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h colorHandler) WithGroup(name string) slog.Handler {
	h.harness = name
	return h
}
