package manifest

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	osuser "os/user"
	"strings"
)

// A manifest defines how to run tests on a specific harness:
//   - Input: define flags that are accepted by the CLI and passed to the
//     entrypoint.
//   - Setup: install packages, load kernel modules, and create directories.
//   - Runtime: check users, groups, files and env vars, and clone git repos.
//   - Output: where the results are stored.
//   - Entrypoint: the function which starts the test.
//   - Cleanup: the function which removes temporary run-specific state.
type Manifest struct {
	Inputs     []Input
	Setup      SysSetup
	Runtime    Runtime
	Output     string
	Entrypoint func(m *Manifest, inputs map[string]FlagValue) (Report, error)
	Cleanup    func(m *Manifest) (int, error)
}

type Input struct {
	Name        string
	Description string
	Default     string
}

func NewInput(name, description string) Input {
	return Input{
		Name:        name,
		Description: description,
	}
}

func (i Input) WithDefault(defaultValue string) Input {
	i.Default = defaultValue
	return i
}

type SysSetup struct {
	Apt []string
	// Snap  []string
	Users      []User
	Dirs       []string
	Modules    []string
	KnownHosts []string
}

type User struct {
	Name   string
	Groups []string
}

type Runtime struct {
	User   string
	Groups []string
	Env    map[string]bool
	Git    []Git
	Files  []string
}

type Git struct {
	Url  string
	Dest string
}

type Report struct {
	Status  int               `json:"status"`
	Outputs map[string]string `json:"outputs"`
}

// A FlagValue represents the string value of an input
// defined by a manifest. It is designed to be a safe way
// to dereference strings from the golang "flag" package.

type FlagValue struct {
	v *string
}

func NewFlagValue(v *string) FlagValue {
	return FlagValue{
		v: v,
	}
}

func (f FlagValue) Get() string {
	value := ""
	if f.v != nil {
		value = *f.v
	}
	return value
}

func Setup(m *Manifest) (int, error) {
	log.Print("Starting harness setup")
	// if m.Setup.OSCodename != "" {
	// 	log.Printf("Checking if OS Version matches: %s\n", m.Setup.OSCodename)
	// 	codename, err := ubuntuCodename()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if codename != m.Setup.OSCodename {
	// 		return fmt.Errorf("Check failed! Current version: %s", codename)
	// 	}
	// }

	log.Printf("Installing packages: %v\n", m.Setup.Apt)

	// needsRoot := len(m.Setup.Apt) != 0 || len(m.Setup.Snap) != 0
	isRoot := os.Geteuid() == 0
	if !isRoot {
		return -1, fmt.Errorf("must run as root")
	}

	sh := Shell{}

	if len(m.Setup.Apt) != 0 {
		st, err := sh.CmdEnv(
			[]string{"DEBIAN_FRONTEND=noninteractive"},
			append([]string{"apt-get", "install", "-y"}, m.Setup.Apt...)...,
		)

		if err != nil {
			return -1, err
		}

		if st != 0 {
			return -1, fmt.Errorf("Failed to install apt packages. Exit status: %d", st)
		}
	}

	if len(m.Setup.Users) != 0 {
		log.Printf("Setting up users")
		for _, u := range m.Setup.Users {
			_, err := osuser.Lookup(u.Name)
			if err != nil {
				var unknown osuser.UnknownUserError
				if !errors.As(err, &unknown) {
					return -1, fmt.Errorf("cannot look up user %s: %w", u.Name, err)
				}
				log.Printf("  Creating user %s", u.Name)
				st, err := sh.Cmd("useradd", "-m", u.Name)
				if err != nil {
					return -1, fmt.Errorf("cannot create user %s: %w", u.Name, err)
				}
				if st != 0 {
					return -1, fmt.Errorf("cannot create user %s: exit status %d", u.Name, st)
				}
			}
			if len(u.Groups) > 0 {
				log.Printf("  Adding user %s to groups %v", u.Name, u.Groups)
				st, err := sh.Cmd("usermod", "-aG", strings.Join(u.Groups, ","), u.Name)
				if err != nil {
					return -1, fmt.Errorf("cannot add user %s to groups: %w", u.Name, err)
				}
				if st != 0 {
					return -1, fmt.Errorf("cannot add user %s to groups: exit status %d", u.Name, st)
				}
			}
		}
	}

	for _, module := range m.Setup.Modules {
		log.Printf("Adding kernel modules: %v\n", m.Setup.Modules)
		st, err := sh.Cmd("modprobe", module)
		if err != nil {
			return -1, fmt.Errorf("cannot load kernel module %s: %w", module, err)
		}
		if st != 0 {
			return -1, fmt.Errorf("cannot load kernel module %s: exit status %d", module, st)
		}
	}

	if len(m.Setup.Dirs) != 0 {
		log.Printf("Creating directories: %s", m.Setup.Dirs)
		st, err := sh.Cmd(append([]string{"mkdir", "-p"}, m.Setup.Dirs...)...)

		if err != nil {
			return -1, err
		}

		if st != 0 {
			return -1, fmt.Errorf("Failed to create directories. Exit status: %d", st)
		}
	}

	if len(m.Setup.KnownHosts) != 0 {
		log.Printf("Setting up SSH known hosts: %v", m.Setup.KnownHosts)
		path := "/etc/ssh/ssh_known_hosts"
		contents, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return -1, err
		}

		added := buildNewEntries(contents, m.Setup.KnownHosts)
		if added != "" {
			err = appendToFile(path, added)
			if err != nil {
				return -1, fmt.Errorf("cannot install SSH host keys: %w", err)
			}
		}
	}

	log.Print("Setup complete!")
	return 0, nil
}

func Run(m *Manifest, inputs map[string]FlagValue) (Report, error) {
	var report Report

	if m.Runtime.User != "" {
		log.Printf("Checking if user is: %s\n", m.Runtime.User)
		id, err := GetEuid(m.Runtime.User)
		if err != nil {
			return report, err
		}
		if id != os.Geteuid() {
			return report, fmt.Errorf("Command must run as user %s", m.Runtime.User)
		}
	}

	for _, group := range m.Runtime.Groups {
		log.Printf("Checking if user belongs to groups: %v\n", m.Runtime.Groups)
		ok, err := CurrentUserInGroup(group)
		if err != nil {
			return report, err
		}
		if !ok {
			return report, fmt.Errorf("User must be part of group %s", group)
		}
	}

	for _, path := range m.Runtime.Files {
		log.Printf("Checking if user can access files: %v\n", m.Runtime.Files)
		info, err := os.Stat(path)
		if err != nil {
			return report, fmt.Errorf("required file %s: %w", path, err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			file, err := os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				return report, fmt.Errorf("cannot open required device %s: %w", path, err)
			}
			file.Close()
		}
	}

	if len(m.Runtime.Env) != 0 {
		log.Printf("Checking if env vars are supplied: %v\n", m.Runtime.Env)
		missing := []string{}
		for name := range m.Runtime.Env {
			if val := os.Getenv(name); val == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return report, fmt.Errorf("Missing env vars: %v", missing)
		}
	}

	sh := Shell{}
	if len(m.Runtime.Git) != 0 {
		log.Printf("Cloning git repos...")
		for _, repo := range m.Runtime.Git {
			log.Printf("Cloning %s...", repo.Url)
			st, err := gitClone(&sh, repo.Url, repo.Dest, true)
			if err != nil {
				return report, fmt.Errorf("cannot clone %s: %w (configure Git access for the run user if required)", repo.Url, err)
			}

			if st != 0 {
				return report, fmt.Errorf("cannot clone %s: exit status %d (configure Git access for the run user if required)", repo.Url, st)
			}
		}
	}

	log.Printf("Executing entrypoint")
	report, err := m.Entrypoint(m, inputs)
	if err != nil {
		return report, err
	}

	return report, nil
}

func Cleanup(m *Manifest) (int, error) {
	if m.Runtime.User != "" {
		id, err := GetEuid(m.Runtime.User)
		if err != nil {
			return -1, err
		}
		if id != os.Geteuid() {
			return -1, fmt.Errorf("Command must run as user %s", m.Runtime.User)
		}
	}

	_, err := m.Cleanup(m)
	if err != nil {
		return -1, err
	}
	return 0, nil
}

func Output(m *Manifest) {
	output := "."
	if m.Output != "" {
		output = m.Output
	}
	fmt.Println(output)
}

func gitClone(s *Shell, url, dest string, shallow bool) (int, error) {
	if DirExists(dest) {
		slog.Warn("Directory already exists, skipping.", "dir", dest)
		return 0, nil
	}

	args := []string{"git", "clone", url, dest}
	if shallow {
		args = append(args, "--depth=1")
	}
	return s.CmdEnv([]string{"GIT_TERMINAL_PROMPT=0"}, args...)
}
