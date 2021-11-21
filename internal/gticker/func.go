// Package gticker provides ticker abstractions.
package gticker

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// Func provides a callback wrapper around ticker that is also pauseable. The
// ticker is ran inside the main loop.
type Func struct {
	F func()
	D time.Duration

	h glib.SourceHandle
}

func (f *Func) IsStarted() bool { return f.h != 0 }

func (f *Func) Start() {
	if f.h != 0 {
		return
	}

	f.h = glib.TimeoutAdd(uint(f.D.Milliseconds()), func() bool {
		f.F()
		return true
	})
	f.F()
}

func (f *Func) Stop() {
	if f.h != 0 {
		glib.SourceRemove(f.h)
		f.h = 0
	}
}
