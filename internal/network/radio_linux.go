//go:build linux

package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LinkOps separates the Linux radio lifecycle from the command fallback.
// Tests supply empty radio callbacks, so even tests on RM1 cannot touch SDIO.
type LinkOps struct {
	Run     func([]string) error
	Ensure  func(string)
	Start   func() error
	Stop    func()
	AfterUp func()
}

type radio struct {
	netRoot, driverDir, devicesDir string
	stat                           func(string) (os.FileInfo, error)
	readDir                        func(string) ([]os.DirEntry, error)
	writeFile                      func(string, []byte, os.FileMode) error
	now                            func() time.Time
	sleep                          func(time.Duration)
}

func systemRadio() radio {
	return radio{"/sys/class/net", "/sys/bus/sdio/drivers/brcmfmac", "/sys/bus/sdio/devices", os.Stat, os.ReadDir, os.WriteFile, time.Now, time.Sleep}
}

func (r radio) supported() bool { _, err := r.stat(r.driverDir); return err == nil }
func (r radio) bind() {
	entries, err := r.readDir(r.devicesDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.Contains(name, ":") {
			continue
		}
		// Leave devices bound to another driver alone.
		if _, err := r.stat(filepath.Join(r.devicesDir, name, "driver")); err == nil {
			continue
		}
		_ = r.writeFile(filepath.Join(r.driverDir, "bind"), []byte(name), 0o200)
	}
}
func (r radio) ensure(iface string) {
	if _, err := r.stat(filepath.Join(r.netRoot, iface)); err == nil {
		return
	}
	if !r.supported() {
		return
	}
	r.bind()
	deadline := r.now().Add(5 * time.Second)
	for r.now().Before(deadline) {
		if _, err := r.stat(filepath.Join(r.netRoot, iface)); err == nil {
			return
		}
		r.sleep(100 * time.Millisecond)
	}
}
func (r radio) unbind() {
	entries, err := r.readDir(r.driverDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ":") {
			_ = r.writeFile(filepath.Join(r.driverDir, "unbind"), []byte(entry.Name()), 0o200)
		}
	}
}

func EnsureInterface(opts Options) {
	if len(opts.WiFiUpCommand) == 0 {
		systemRadio().ensure(opts.Interface)
	}
}

func defaultLinkOps(run func([]string) error) LinkOps {
	r := systemRadio()
	return LinkOps{Run: run, Ensure: r.ensure,
		AfterUp: func() {
			if r.supported() {
				_ = run([]string{"systemctl", "start", "wpa_supplicant.service"})
			}
		},
		Start: func() error {
			if !r.supported() {
				return nil
			}
			if _, err := exec.LookPath("rfkill"); err == nil {
				if err := run([]string{"rfkill", "unblock", "wifi"}); err != nil {
					return fmt.Errorf("rfkill unblock: %w", err)
				}
			}
			return nil
		},
		Stop: func() {
			if !r.supported() {
				return
			}
			_ = run([]string{"systemctl", "stop", "wpa_supplicant.service"})
			if _, err := exec.LookPath("rfkill"); err == nil {
				_ = run([]string{"rfkill", "block", "wifi"})
			}
			r.unbind()
		}}
}
