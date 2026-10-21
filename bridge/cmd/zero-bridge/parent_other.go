//go:build !unix

package main

import "errors"

func exitWithParentSupported() error {
	return errors.New("exit with parent is not implemented")
}

// armExitWithParent is the non-unix stub. The flag is refused so a Windows
// build does not pretend the parent is watched.
func armExitWithParent(func()) error {
	return exitWithParentSupported()
}
