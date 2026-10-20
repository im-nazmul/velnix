package platform

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Facts struct {
	OS                  string   `json:"os"`
	Architecture        string   `json:"architecture"`
	Distribution        string   `json:"distribution"`
	DistributionVersion string   `json:"distribution_version"`
	Kernel              string   `json:"kernel"`
	WSL                 bool     `json:"wsl"`
	PID1                string   `json:"pid1"`
	Systemd             bool     `json:"systemd"`
	SystemdUserRuntime  bool     `json:"systemd_user_runtime"`
	PackageManagers     []string `json:"package_managers"`
	Warnings            []string `json:"warnings"`
}

// Detector reads metadata only. It never invokes discovered executables.
type Detector struct{ Root string }

func (d Detector) Detect(ctx context.Context) (Facts, error) {
	f := Facts{OS: runtime.GOOS, Architecture: runtime.GOARCH, PackageManagers: []string{}, Warnings: []string{}}
	read := func(path string) string {
		if ctx.Err() != nil {
			return ""
		}
		file, err := os.Open(filepath.Join(d.Root, path))
		if err != nil {
			return ""
		}
		defer file.Close()
		b, _ := io.ReadAll(io.LimitReader(file, 65536))
		return strings.TrimSpace(string(b))
	}
	release := read("/etc/os-release")
	if release == "" {
		f.Warnings = append(f.Warnings, "Distribution metadata unavailable")
	}
	for _, line := range strings.Split(release, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "\"'")
		switch key {
		case "ID":
			f.Distribution = value
		case "VERSION_ID":
			f.DistributionVersion = value
		}
	}
	f.Kernel = read("/proc/sys/kernel/osrelease")
	f.WSL = strings.Contains(strings.ToLower(f.Kernel), "microsoft") || strings.Contains(strings.ToLower(read("/proc/version")), "microsoft")
	f.PID1 = read("/proc/1/comm")
	info, err := os.Stat(filepath.Join(d.Root, "/run/systemd/system"))
	f.Systemd = f.PID1 == "systemd" && err == nil && info.IsDir()
	// This is an availability hint, not a successful user-bus connection.
	if x := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(x) {
		info, err = os.Stat(filepath.Join(d.Root, x, "systemd/private"))
		f.SystemdUserRuntime = err == nil && info.Mode()&os.ModeSocket != 0
	}
	for _, manager := range []string{"apt-get", "dnf", "pacman", "zypper", "apk"} {
		for _, dir := range []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin"} {
			info, err := os.Stat(filepath.Join(d.Root, dir, manager))
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				f.PackageManagers = append(f.PackageManagers, manager)
				break
			}
		}
	}
	sort.Strings(f.PackageManagers)
	if f.WSL {
		f.Warnings = append(f.Warnings, "WSL networking mode, Windows DNS and browser trust have not been verified")
	}
	if !f.Systemd {
		f.Warnings = append(f.Warnings, "System systemd manager not detected; service lifecycle support is unavailable")
	}
	return f, ctx.Err()
}
