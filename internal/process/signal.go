package process

import "os"

// SignalCause preserves the terminal signal when a command context is canceled.
type SignalCause struct{ Signal os.Signal }

func (s *SignalCause) Error() string { return "received " + s.Signal.String() }
