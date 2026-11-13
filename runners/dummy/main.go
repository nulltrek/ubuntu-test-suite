package main

import (
	"flag"
	"fmt"
	"os"

	m "canonical.com/hwrap/manifest"
)

func usageMsg() {
	fmt.Fprintf(os.Stderr, "Usage: %s setup \n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s run [manifest input flags] \n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s cleanup\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	manifest := &m.Manifest{
		Inputs: []m.Input{
			m.NewInput("input", "The main input string").WithDefault("dummy"),
		},
		Setup:   m.SysSetup{},
		Output:  "results",
		Runtime: m.Runtime{},
		Entrypoint: func(_ *m.Manifest, inputs map[string]m.FlagValue) (m.Report, error) {
			r := m.Report{
				Status: 0,
				Outputs: map[string]string{
					"input": inputs["input"].Get(),
				},
			}
			return r, nil
		},
		Cleanup: func(_ *m.Manifest) (int, error) {
			return 0, nil
		},
	}

	m.Start(manifest, "dummy", usageMsg)
}
