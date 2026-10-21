//go:build unix

package main

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// parentPollInterval is how often the process checks the parent it recorded
// at start. A thread death inside that parent does not change the pid, so
// this stays up when a multithreaded parent loses a thread.
const parentPollInterval = 200 * time.Millisecond

func exitWithParentSupported() error { return nil }

// armExitWithParent shuts down through stop when the parent recorded at
// start is gone. stop is the SIGTERM cancel from signal.NotifyContext.
func armExitWithParent(stop func()) error {
	parent := os.Getppid()
	go func() {
		tick := time.NewTicker(parentPollInterval)
		defer tick.Stop()
		for range tick.C {
			if parentGone(parent) {
				stop()
				return
			}
		}
	}()
	return nil
}

func parentGone(parent int) bool {
	if parent <= 0 {
		return false
	}
	if os.Getppid() != parent {
		return true
	}
	err := syscall.Kill(parent, 0)
	return errors.Is(err, syscall.ESRCH)
}
