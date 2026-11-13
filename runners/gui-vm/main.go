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
			m.NewInput("iso", "The ISO to test"),
			m.NewInput("test", "The test to run"),
		},
		Setup: m.SysSetup{
			Users: []m.User{
				{
					Name:   "ubuntu",
					Groups: []string{"kvm"},
				},
			},
			Apt: []string{
				"build-essential",
				"clang",
				"libxkbcommon-dev",
				"tesseract-ocr",
				"sshpass",
				"socat",
				"ovmf",
				"qemu-system-x86",
				"qemu-utils",
				"python3-venv",
				"python3-dev",
				"acl",
			},
			Modules: []string{"vhost_vsock"},
		},
		Runtime: m.Runtime{
			User: "ubuntu",
			Groups: []string{
				"kvm",
			},
			Files: []string{"/dev/kvm", "/dev/vhost-vsock"},
			Git: []m.Git{
				{
					Url:  "https://github.com/canonical/hardware-installer-testing.git",
					Dest: "hardware-installer-testing",
				},
				{
					Url:  "https://github.com/canonical/yarf.git",
					Dest: "yarf",
				},
				{
					Url:  "https://github.com/canonical/ubuntu-gui-testing.git",
					Dest: "ubuntu-gui-testing",
				},
			},
		},
		Entrypoint: func(_ *m.Manifest, inputs map[string]m.FlagValue) (m.Report, error) {
			report := m.Report{Status: 1}
			iso := inputs["iso"].Get()
			if iso == "" {
				return report, fmt.Errorf("No image selected")
			}

			test := inputs["test"].Get()
			if test == "" {
				return report, fmt.Errorf("No test selected")
			}

			sh := m.Shell{}
			sh.Cmd("mkdir", "-p", "storage")

			sh.Cmd("python3", "-m", "venv", "uv-venv")
			sh.Cmd("uv-venv/bin/pip", "install", "uv")

			sh.Cmd("uv-venv/bin/uv", "sync", "--directory", "yarf")
			sh.Cmd("uv-venv/bin/uv", "pip", "install",
				"--python", "yarf/.venv/bin/python3",
				"-r", "hardware-installer-testing/requirements.txt",
			)

			sh.CmdEnv(
				[]string{
					"VIRTUAL_ENV=" + m.AbsPath("yarf/.venv"),
					"PATH=" + m.AbsPath("yarf/.venv/bin") + ":" + os.Getenv("PATH"),
				},
				m.AbsPath("yarf/.venv/bin/python3"),
				"hardware-installer-testing/runner/spawn_and_run_test_suite.py",
				"--test-suite", "ubuntu-gui-testing/tests/desktop-installer",
				"--iso", iso,
				"--qemu-args-json", "hardware-installer-testing/runner/qemu-args.json",
				"--cleanup-storage",
				"--storage-prefix", "./storage",
				"--suite", test, // "Stonking.Entire-Disk",
			)

			return m.Report{Status: sh.Status}, sh.Err
		},
	}

	m.Start(manifest, "gui-vm", usageMsg)
}
