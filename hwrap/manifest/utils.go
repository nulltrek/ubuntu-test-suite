package manifest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Stateful script executor which stops running commands when an
// error is encountered. Makes it possible to define a list of
// commands to execute without having to check for errors each time.
// Example:
//
// sh := shell{}
// sh.cmd("mv", "src/file", "dest/file")
// sh.cmd("chmod", "u+x", "dest/file")
// sh.cmd("ls", "dest")

//	if sh.err != nil || sh.status != 0 {
//		fmt.Printf("Failure!")
//	}
//
// If an error happens while running the second command, it gets recorded
// into sh.err or sh.status, and the third command is just never executed.
type Shell struct {
	Err    error
	Status int
	// cwd    string
}

// cmd runs a command with arguments.
func (s *Shell) Cmd(args ...string) (int, error) {
	return s.CmdEnv([]string{}, args...)
}

// cmdEnv runs a command with arguments, providing it with a list of env vars.
func (s *Shell) CmdEnv(env []string, args ...string) (int, error) {
	if s.Err != nil || s.Status > 0 {
		return s.Status, s.Err
	}

	cmd := exec.Command(args[0], args[1:]...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			s.Status = exitErr.ExitCode()
			return s.Status, nil
		}
		s.Err = fmt.Errorf("error while executing command %s: %w", args[0], err)
		return -1, s.Err
	}

	return 0, nil
}

// Linux/Ubuntu utilities

func UbuntuCodename() (string, error) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "VERSION_CODENAME="); ok {
			return strings.Trim(v, `"`), nil
		}
	}
	return "", fmt.Errorf("no VERSION_CODENAME in /etc/os-release")
}

func GetEuid(username string) (int, error) {
	u, err := osuser.Lookup(username)
	if err != nil {
		return -1, err
	}

	id, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func AbsPath(rel string) string {
	wd, err := os.Getwd()
	if err != nil {
		return rel
	}
	return filepath.Join(wd, rel)
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func CurrentUser() string {
	if u, err := osuser.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

func CurrentUserInGroup(name string) (bool, error) {
	g, err := osuser.LookupGroup(name)
	if err != nil {
		return false, err
	}
	groups, err := os.Getgroups()
	if err != nil {
		return false, err
	}
	for _, gid := range groups {
		if strconv.Itoa(gid) == g.Gid {
			return true, nil
		}
	}
	return false, nil
}

func RealPath(rel string) (string, error) {
	abs, err := filepath.Abs(rel)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// buildNewEntries given some file contents and some new lines to
// add, build a newline-separated string containing only the lines
// not already present in the original contents.
func buildNewEntries(contents []byte, entries []string) string {
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(contents), "\n") {
		seen[line] = true
	}

	var added strings.Builder
	// If the original file doesn't end with a newline, we add
	// one before our entries
	if len(contents) > 0 && contents[len(contents)-1] != '\n' {
		added.WriteByte('\n')
	}

	count := 0
	for _, entry := range entries {
		if entry == "" || seen[entry] {
			continue
		}

		count++
		added.WriteString(entry + "\n")
		seen[entry] = true
	}

	result := ""
	if count > 0 {
		result = added.String()
	}

	return result
}

func appendToFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
