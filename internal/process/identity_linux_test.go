//go:build linux

package process

import (
	"golang.org/x/sys/unix"
	"os"
	"testing"
)

func TestIdentityRejectsReuse(t *testing.T) {
	i, e := Observe(os.Getpid())
	if e != nil {
		t.Fatal(e)
	}
	fd, e := Handle(i)
	if e != nil {
		t.Fatal(e)
	}
	unix.Close(fd)
	i.Start = "1"
	if e = Signal(i, 0); e == nil {
		t.Fatal("accepted mismatched process identity")
	}
	i.PID = 0
	if e = Signal(i, unix.SIGTERM); e == nil {
		t.Fatal("accepted missing identity")
	}
}
