//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestExitWithParent starts the bridge as the child of an intermediate
// process, kills that process with SIGKILL, and checks the unix socket.
// With --exit-with-parent the bridge must leave and the socket must
// disappear. Without the flag the bridge must keep running.
func TestExitWithParent(t *testing.T) {
	fake := buildAcpFake(t)
	t.Run("flag", func(t *testing.T) {
		proc := startUnderParent(t, fake, true)
		killParent(t, proc)
		if !waitDown(proc, 3*time.Second) {
			t.Fatalf("bridge still up after parent SIGKILL pid=%d socket=%t stderr=%s", proc.bridge, socketExists(proc.sock), tailFile(proc.stderr))
		}
	})
	t.Run("default", func(t *testing.T) {
		proc := startUnderParent(t, fake, false)
		killParent(t, proc)
		time.Sleep(time.Second)
		if !processAlive(proc.bridge) || !socketExists(proc.sock) {
			t.Fatalf("bridge left without the flag pid=%d socket=%t stderr=%s", proc.bridge, socketExists(proc.sock), tailFile(proc.stderr))
		}
	})
}

// TestParentWatchHelper is the intermediate parent. The real test re-execs
// this binary and sets ZB_PARENT_HELPER. A normal go test leaves it idle.
func TestParentWatchHelper(t *testing.T) {
	if os.Getenv("ZB_PARENT_HELPER") != "1" {
		return
	}
	stderr, err := os.OpenFile(os.Getenv("ZB_BRIDGE_STDERR"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBridgeUnderParent$")
	cmd.Env = append(envWithout("ZB_PARENT_HELPER", "ZB_BRIDGE_CHILD"),
		"ZB_BRIDGE_CHILD=1",
		"ZB_BRIDGE_ARGFILE="+os.Getenv("ZB_BRIDGE_ARGFILE"),
	)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("ZB_CHILD_PIDFILE"), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("ZB_CHILD_READY"), []byte("1\n"), 0o600); err != nil {
		os.Exit(2)
	}
	select {}
}

// TestBridgeUnderParent is the bridge process under the helper. It enters
// the real runArgs path. A normal go test leaves it idle.
func TestBridgeUnderParent(t *testing.T) {
	if os.Getenv("ZB_BRIDGE_CHILD") != "1" {
		return
	}
	raw, err := os.ReadFile(os.Getenv("ZB_BRIDGE_ARGFILE"))
	if err != nil {
		os.Exit(2)
	}
	var args []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line != "" {
			args = append(args, line)
		}
	}
	os.Exit(runArgs(args, os.Stderr))
}

type parentProc struct {
	helper *exec.Cmd
	bridge int
	sock   string
	stderr string
	argv0  string
}

func startUnderParent(t *testing.T, fake string, exitWithParent bool) *parentProc {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "zb75")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ws := filepath.Join(dir, "ws")
	state := filepath.Join(dir, "state")
	if err := os.MkdirAll(ws, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, "b.sock")
	args := []string{
		"--listen", "unix:" + sock,
		"--agent-cmd", fake,
		"--profile", "generic",
		"--workspace", ws,
		"--state-dir", state,
		"--init-timeout", "5s",
	}
	if exitWithParent {
		args = append(args, "--exit-with-parent")
	}
	argFile := filepath.Join(dir, "args")
	if err := os.WriteFile(argFile, []byte(strings.Join(args, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	proc := &parentProc{
		sock:   sock,
		stderr: filepath.Join(dir, "err"),
		argv0:  os.Args[0],
	}
	pidFile := filepath.Join(dir, "pid")
	readyFile := filepath.Join(dir, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestParentWatchHelper$")
	cmd.Env = append(envWithout("ZB_PARENT_HELPER", "ZB_BRIDGE_CHILD"),
		"ZB_PARENT_HELPER=1",
		"ZB_BRIDGE_ARGFILE="+argFile,
		"ZB_CHILD_PIDFILE="+pidFile,
		"ZB_CHILD_READY="+readyFile,
		"ZB_BRIDGE_STDERR="+proc.stderr,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	proc.helper = cmd
	t.Cleanup(func() {
		if proc.bridge > 0 {
			stopIfCommand(proc.bridge, proc.argv0)
		}
		if cmd.Process != nil {
			stopIfCommand(cmd.Process.Pid, proc.argv0)
		}
	})
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyFile); err == nil {
			pid, err := readPID(pidFile)
			if err == nil && socketExists(sock) {
				proc.bridge = pid
				return proc
			}
		}
		if !processAlive(cmd.Process.Pid) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("bridge did not listen stderr=%s", tailFile(proc.stderr))
	return nil
}

func killParent(t *testing.T, proc *parentProc) {
	t.Helper()
	if err := syscall.Kill(proc.helper.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = proc.helper.Wait()
}

func waitDown(proc *parentProc, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if !processAlive(proc.bridge) && !socketExists(proc.sock) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !processAlive(proc.bridge) && !socketExists(proc.sock)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func socketExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPID(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func envWithout(drop ...string) []string {
	skip := map[string]bool{}
	for _, key := range drop {
		skip[key] = true
	}
	var out []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if skip[key] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func stopIfCommand(pid int, want string) {
	if pid <= 0 || !processAlive(pid) {
		return
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || !strings.Contains(string(b), want) {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func tailFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 400 {
		return s[len(s)-400:]
	}
	return s
}
