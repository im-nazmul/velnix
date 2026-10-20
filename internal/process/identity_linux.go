//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"velnix/internal/domain"
)

type Identity struct {
	PID        int    `json:"pid"`
	Boot       string `json:"boot_id"`
	Start      string `json:"start_ticks"`
	Executable string `json:"executable"`
}

func Observe(pid int) (Identity, error) {
	var i Identity
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if e != nil {
		return i, e
	}
	stat, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return i, e
	}
	end := strings.LastIndex(string(stat), ")")
	if end < 0 {
		return i, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(stat)[end+1:])
	if len(fields) < 20 {
		return i, fmt.Errorf("short process stat")
	}
	exe, e := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if e != nil {
		return i, e
	}
	return Identity{pid, strings.TrimSpace(string(b)), fields[19], strings.TrimSuffix(exe, " (deleted)")}, nil
}

// Handle pins the kernel process before comparing identity; PID reuse cannot
// redirect a later signal. Kernels without pidfds fail closed.
func Handle(i Identity) (int, error) {
	if i.PID <= 1 || i.Start == "" || i.Boot == "" {
		return -1, domain.E(domain.Conflict, "missing owned process identity")
	}
	if _, e := strconv.ParseUint(i.Start, 10, 64); e != nil {
		return -1, e
	}
	fd, e := unix.PidfdOpen(i.PID, 0)
	if e != nil {
		return -1, e
	}
	now, e := Observe(i.PID)
	if e != nil || now != i {
		unix.Close(fd)
		return -1, domain.E(domain.Conflict, "process identity changed; refusing signal")
	}
	return fd, nil
}
func Signal(i Identity, sig unix.Signal) error {
	fd, e := Handle(i)
	if e != nil {
		return e
	}
	defer unix.Close(fd)
	return unix.PidfdSendSignal(fd, sig, nil, 0)
}
func Alive(i Identity) bool {
	fd, e := Handle(i)
	if e != nil {
		return false
	}
	defer unix.Close(fd)
	return unix.PidfdSendSignal(fd, 0, nil, 0) == nil
}

// CheckedAlive distinguishes a gone/reused PID from an inaccessible or
// unexpectedly exec-replaced process; the latter requires manual review.
func CheckedAlive(i Identity) (bool, error) {
	if i.PID == 0 {
		return false, nil
	}
	now, e := Observe(i.PID)
	if os.IsNotExist(e) || errors.Is(e, unix.ESRCH) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if now.Boot != i.Boot || now.Start != i.Start {
		return false, nil
	}
	if now.Executable != i.Executable {
		return false, domain.E(domain.Conflict, "owned process executable changed; manual review required")
	}
	fd, e := Handle(i)
	if errors.Is(e, unix.ESRCH) || os.IsNotExist(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	defer unix.Close(fd)
	return true, nil
}

// Children snapshots identities rather than retaining reusable child PIDs.
func Children(i Identity) ([]Identity, error) {
	fd, e := Handle(i)
	if e != nil {
		return nil, e
	}
	defer unix.Close(fd)
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", i.PID, i.PID))
	if e != nil {
		return nil, e
	}
	var out []Identity
	for _, field := range strings.Fields(string(b)) {
		pid, e := strconv.Atoi(field)
		if e != nil {
			return nil, e
		}
		child, e := Observe(pid)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		out = append(out, child)
	}
	return out, nil
}

// Probe probes the same-UID pidfd capability before workload launch.
func Probe() (int, error) {
	i, e := Observe(os.Getpid())
	if e != nil {
		return -1, e
	}
	return Handle(i)
}
