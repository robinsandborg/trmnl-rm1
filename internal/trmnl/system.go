package trmnl

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func commandString(parts []string) string {
	return strings.Join(parts, " ")
}

func runCommand(parts []string) error {
	return runCommandWithEnv(parts, nil)
}

func runCommandWithEnv(parts []string, env []string) error {
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("%s: %w", commandString(parts), err)
		}
		return fmt.Errorf("%s: %s", commandString(parts), msg)
	}
	return nil
}

func outputCommand(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", commandString(parts), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func firstSuccessful(commands ...[]string) error {
	var errs []error
	for _, cmd := range commands {
		if len(cmd) == 0 {
			continue
		}
		if err := runCommand(cmd); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return errors.New("no commands available")
	}
	return errors.Join(errs...)
}
