//go:build linux

package server

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func listenUnix(path string) (net.Listener, error) {
	if err := prepareUnix(path); err != nil {
		return nil, err
	}
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	old := syscall.Umask(0077)
	err = syscall.Bind(fd, &syscall.SockaddrUnix{Name: path})
	syscall.Umask(old)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	// fchmod on a socket fd does not change the inode mode on this kernel.
	// chmod applies only after Lstat has shown a socket, not a symlink.
	if err := chmodOwnSocket(path, 0600); err != nil {
		syscall.Close(fd)
		_ = removeUnixSocket(path)
		return nil, err
	}
	if err := syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		syscall.Close(fd)
		_ = removeUnixSocket(path)
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	ln, err := net.FileListener(f)
	f.Close()
	if err != nil {
		_ = removeUnixSocket(path)
		return nil, err
	}
	if ul, ok := ln.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	return ln, nil
}

func chmodOwnSocket(path string, mode os.FileMode) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeSocket == 0 {
		return errors.New("listen path is not a stale socket")
	}
	return os.Chmod(path, mode)
}

func prepareUnix(path string) error {
	if len(path) >= 108 {
		return errors.New("unix listen path is too long")
	}
	dir, name := filepath.Split(path)
	dir = filepath.Clean(dir)
	if name == "" || name == "." || name == ".." {
		return errors.New("unix listen path must name a socket")
	}
	if err := ensureParent(dir); err != nil {
		return err
	}
	return clearStaleSocket(dir, name, path)
}

func ensureParent(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return createDir0700(dir)
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("listen directory is a symlink")
	}
	if !fi.IsDir() {
		return errors.New("listen directory is not a directory")
	}
	f, err := openDirNoFollow(dir)
	if err != nil {
		return errors.New("listen directory is a symlink")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if err := refuseOpenMode(st.Mode().Perm()); err != nil {
		return err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("listen directory owner does not match")
	}
	return ownerMatches(sys.Uid)
}

func createDir0700(dir string) error {
	old := syscall.Umask(0077)
	err := os.Mkdir(dir, 0700)
	syscall.Umask(old)
	if err != nil {
		return err
	}
	f, err := openDirNoFollow(dir)
	if err != nil {
		return errors.New("listen directory is a symlink")
	}
	defer f.Close()
	if err := syscall.Fchmod(int(f.Fd()), 0700); err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	return refuseOpenMode(fi.Mode().Perm())
}

func openDirNoFollow(dir string) (*os.File, error) {
	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), dir), nil
}

func clearStaleSocket(dir, name, path string) error {
	f, err := openDirNoFollow(dir)
	if err != nil {
		return errors.New("listen directory is a symlink")
	}
	defer f.Close()
	fi, err := lstatName(f, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := refuseSocketEntry(fi); err != nil {
		return err
	}
	if connectUnix(path) == nil {
		return errors.New("socket is already listening")
	}
	again, err := lstatName(f, name)
	if err != nil {
		return err
	}
	if err := refuseSocketEntry(again); err != nil {
		return err
	}
	return syscall.Unlinkat(int(f.Fd()), name)
}

func connectUnix(path string) error {
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return err
	}
	c.Close()
	return nil
}

func removeUnixSocket(path string) error {
	if path == "" {
		return nil
	}
	dir, name := filepath.Split(path)
	dir = filepath.Clean(dir)
	f, err := openDirNoFollow(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := lstatName(f, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := refuseSocketEntry(fi); err != nil {
		return err
	}
	return syscall.Unlinkat(int(f.Fd()), name)
}

// lstatName reads one directory entry through the already-open directory
// descriptor, so the final component is not followed.
func lstatName(dir *os.File, name string) (os.FileInfo, error) {
	if _, err := dir.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	entries, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}
	for _, fi := range entries {
		if fi.Name() == name {
			return fi, nil
		}
	}
	return nil, os.ErrNotExist
}

func refuseSocketEntry(fi os.FileInfo) error {
	if fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("listen path is a symlink")
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return errors.New("listen path is not a stale socket")
	}
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("listen path owner does not match")
	}
	return ownerMatches(sys.Uid)
}

func refuseOpenMode(perm os.FileMode) error {
	if perm&0077 != 0 {
		return errors.New("listen directory mode is too open")
	}
	return nil
}

func ownerMatches(uid uint32) error {
	if uid != uint32(os.Getuid()) {
		return errors.New("listen path owner does not match")
	}
	return nil
}
