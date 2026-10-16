//go:build !linux

package server

import (
	"errors"
	"net"
)

func listenUnix(string) (net.Listener, error) {
	return nil, errors.New("unix listen is not supported")
}

func removeUnixSocket(string) error { return nil }
