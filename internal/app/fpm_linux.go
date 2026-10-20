//go:build linux

package app

import (
	"context"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"velnix/internal/domain"
	"velnix/internal/process"
	"velnix/internal/providers/nginx"
)

func poolHealth(ctx context.Context, r HTTPRecord) error {
	for {
		if !process.Alive(r.Process) {
			return domain.E(domain.Unavailable, "owned FPM process exited")
		}
		info, e := os.Lstat(r.FPM.Socket)
		if e == nil {
			stat := info.Sys().(*syscall.Stat_t)
			if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) {
				return domain.E(domain.Denied, "FPM socket must be private and owned by the current user")
			}
			workers, err := process.Children(r.Process)
			if err != nil {
				return err
			}
			if len(workers) > 0 {
				conn, err := net.DialTimeout("unix", r.FPM.Socket, 100*time.Millisecond)
				if err == nil {
					conn.Close()
					return nil
				}
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (s HTTPService) routes(projects []domain.Project) (map[string]nginx.PHPRoute, []PoolEvidence, error) {
	routes := map[string]nginx.PHPRoute{}
	var evidence []PoolEvidence
	for _, p := range projects {
		b, e := s.Store.ReadWorkload("fpm-" + p.Name + "-http.json")
		if e != nil {
			return nil, nil, e
		}
		if b == nil {
			continue
		}
		r, e := readRecord(b)
		if e != nil {
			return nil, nil, e
		}
		if r.FPM == nil {
			continue
		}
		if r.Phase != "running" {
			return nil, nil, domain.E(domain.Unavailable, "FPM for %s is unavailable; start it before HTTP start/reload", p.Name)
		}
		alive, e := process.CheckedAlive(r.Process)
		if e != nil {
			return nil, nil, e
		}
		if !alive {
			return nil, nil, domain.E(domain.Unavailable, "FPM for %s exited", p.Name)
		}
		want := r.FPM.Project
		want.PHPCLI = p.PHPCLI
		if !reflect.DeepEqual(want, p) {
			return nil, nil, domain.E(domain.Conflict, "FPM project metadata changed; restart pool %s", p.Name)
		}
		if e = r.FPM.Validate(); e != nil {
			return nil, nil, e
		}
		c, e := s.Store.ReadWorkload("fpm-" + p.Name + "-http.conf")
		if e != nil {
			return nil, nil, e
		}
		if hash(c) != r.ConfigDigest {
			return nil, nil, domain.E(domain.Conflict, "FPM configuration edited externally")
		}
		info, e := os.Lstat(r.FPM.Socket)
		if e != nil {
			return nil, nil, domain.E(domain.Unavailable, "FPM socket unavailable: %v", e)
		}
		st := info.Sys().(*syscall.Stat_t)
		if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || st.Uid != uint32(os.Geteuid()) || r.SocketInode == 0 || st.Ino != r.SocketInode || uint64(st.Dev) != r.SocketDevice {
			return nil, nil, domain.E(domain.Denied, "unsafe FPM socket")
		}
		evidence = append(evidence, PoolEvidence{Name: p.Name, Digest: hash(append(b, data([]uint64{uint64(st.Dev), st.Ino})...))})
		routes[p.Name] = nginx.PHPRoute{Socket: r.FPM.Socket, Profile: r.FPM.Profile, Uploads: r.FPM.Uploads}
	}
	return routes, evidence, nil
}

// Only remove a socket whose identity was durably recorded after readiness.
func cleanupPoolSocket(r HTTPRecord) error {
	if r.FPM == nil {
		return nil
	}
	info, e := os.Lstat(r.FPM.Socket)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	st := info.Sys().(*syscall.Stat_t)
	if r.SocketInode == 0 || info.Mode()&os.ModeSocket == 0 || uint64(st.Dev) != r.SocketDevice || st.Ino != r.SocketInode || st.Uid != uint32(os.Geteuid()) {
		return domain.E(domain.Conflict, "FPM socket identity is unknown or changed; preserve for manual recovery")
	}
	return os.Remove(r.FPM.Socket)
}
