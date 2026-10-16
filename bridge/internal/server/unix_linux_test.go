//go:build linux

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPeerAllowed(t *testing.T) {
	if !peerAllowed(uint32(os.Getuid())) {
		t.Fatal("same uid refused")
	}
	if peerAllowed(uint32(os.Getuid()) + 1) {
		t.Fatal("foreign uid accepted")
	}
}

func TestUnixRefusesForeignPeer(t *testing.T) {
	prev := readPeerUID
	readPeerUID = func(net.Conn) (uint32, error) {
		return uint32(os.Getuid()) + 1, nil
	}
	t.Cleanup(func() { readPeerUID = prev })
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	srv := newUnixServer(t, path, "")
	stop := serveUnix(t, srv, path)
	defer stop()
	req, err := http.NewRequest(http.MethodGet, "http://localhost/v1/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := unixClient(path).Do(req)
	if err == nil {
		res.Body.Close()
		t.Fatalf("foreign peer status %d", res.StatusCode)
	}
}

func TestUnixRefusesPeerCredError(t *testing.T) {
	prev := readPeerUID
	readPeerUID = func(net.Conn) (uint32, error) {
		return 0, errors.New("peer cred")
	}
	t.Cleanup(func() { readPeerUID = prev })
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	srv := newUnixServer(t, path, "")
	stop := serveUnix(t, srv, path)
	defer stop()
	req, err := http.NewRequest(http.MethodGet, "http://localhost/v1/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := unixClient(path).Do(req)
	if err == nil {
		res.Body.Close()
		t.Fatalf("peer cred error status %d", res.StatusCode)
	}
}

func TestUnixRefusesGroupBits(t *testing.T) {
	for _, mode := range []os.FileMode{0750, 0770} {
		t.Run(mode.String(), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "group")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, mode); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "bridge.sock")
			srv := newUnixServer(t, path, "")
			err := serveUntilError(srv)
			if err == nil || !strings.Contains(err.Error(), "too open") {
				t.Fatalf("err %v", err)
			}
			fi, statErr := os.Lstat(dir)
			if statErr != nil || fi.Mode().Perm() != mode {
				t.Fatalf("dir mode changed: %v %#v", statErr, fi)
			}
			if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
				t.Fatal("socket created in a group-open directory")
			}
		})
	}
}

func TestUnixRefusesSwappedOwner(t *testing.T) {
	prev := currentUID
	currentUID = func() int { return os.Getuid() + 1 }
	t.Cleanup(func() { currentUID = prev })

	dir := filepath.Join(t.TempDir(), "parent")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	err := serveUntilError(newUnixServer(t, path, ""))
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("parent err %v", err)
	}
	if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
		t.Fatal("socket created for a foreign directory owner")
	}

	sockDir := filepath.Join(t.TempDir(), "sock")
	if err := os.Mkdir(sockDir, 0700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(sockDir, "bridge.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	err = serveUntilError(newUnixServer(t, sock, ""))
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("socket err %v", err)
	}
	fi, statErr := os.Lstat(sock)
	if statErr != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("existing socket not left in place: %v %#v", statErr, fi)
	}
}

func TestOwnerAndModePredicates(t *testing.T) {
	if err := ownerMatches(uint32(os.Getuid())); err != nil {
		t.Fatal(err)
	}
	if err := ownerMatches(uint32(os.Getuid()) + 1); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if err := refuseOpenMode(0700); err != nil {
		t.Fatal(err)
	}
	if err := refuseOpenMode(0755); err == nil {
		t.Fatal("open directory mode accepted")
	}
}

func TestUnixSocketModeAndShutdown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sockdir")
	path := filepath.Join(dir, "bridge.sock")
	srv := newUnixServer(t, path, "")
	stop := serveUnix(t, srv, path)
	defer stop()
	assertMode(t, dir, 0700)
	assertMode(t, path, 0600)
	body := unixGet(t, path, "/v1/info", nil)
	if strings.Contains(body, path) || strings.Contains(body, "sockdir") {
		t.Fatalf("path leaked %s", body)
	}
	var info struct {
		Listen string `json:"listen"`
	}
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatal(err)
	}
	if info.Listen != "unix" {
		t.Fatalf("listen %q", info.Listen)
	}
	stop()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket remains: %v", err)
	}
}

func TestUnixRefusesOpenDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	srv := newUnixServer(t, path, "")
	err := serveUntilError(srv)
	if err == nil || !strings.Contains(err.Error(), "too open") {
		t.Fatalf("err %v", err)
	}
	fi, statErr := os.Lstat(dir)
	if statErr != nil || fi.Mode().Perm() != 0755 {
		t.Fatalf("dir mode changed: %v %#v", statErr, fi)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("socket created in an open directory")
	}
}

func TestUnixRefusesSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(link, "bridge.sock")
	srv := newUnixServer(t, path, "")
	err := serveUntilError(srv)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err %v", err)
	}
	if _, statErr := os.Lstat(link); statErr != nil {
		t.Fatal(statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(real, "bridge.sock")); !os.IsNotExist(statErr) {
		t.Fatal("followed the directory symlink")
	}
}

func TestUnixRefusesSymlinkSocket(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	if err := os.Symlink("/tmp/zero-bridge-not-a-socket", path); err != nil {
		t.Fatal(err)
	}
	srv := newUnixServer(t, path, "")
	err := serveUntilError(srv)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err %v", err)
	}
	fi, statErr := os.Lstat(path)
	if statErr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink removed: %v %#v", statErr, fi)
	}
}

func TestUnixRefusesRegularFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	srv := newUnixServer(t, path, "")
	err := serveUntilError(srv)
	if err == nil || !strings.Contains(err.Error(), "not a stale socket") {
		t.Fatalf("err %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "keep" {
		t.Fatalf("file changed: %v %q", readErr, got)
	}
}

func TestUnixReplacesStaleSocket(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	srv := newUnixServer(t, path, "")
	stop := serveUnix(t, srv, path)
	defer stop()
	if !strings.Contains(unixGet(t, path, "/v1/info", nil), `"listen":"unix"`) {
		t.Fatal("stale socket was not replaced")
	}
}

func TestUnixRefusesLiveSocket(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	first := newUnixServer(t, path, "")
	stop := serveUnix(t, first, path)
	defer stop()
	second := newUnixServer(t, path, "")
	err := serveUntilError(second)
	if err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(unixGet(t, path, "/v1/info", nil), `"listen":"unix"`) {
		t.Fatal("live socket stopped")
	}
}

func TestUnixAuthOriginAndBearer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bridge.sock")
	origin := "http://bridge.test:8780"
	srv := newUnixServer(t, path, bearerToken)
	srv.cfg.PublicOrigins = []string{origin}
	stop := serveUnix(t, srv, path)
	defer stop()

	noOrigin := unixStatus(t, path, http.MethodGet, "/v1/info", "", "", nil)
	if noOrigin != http.StatusUnauthorized {
		t.Fatalf("open info %d", noOrigin)
	}
	withBearer := unixStatus(t, path, http.MethodGet, "/v1/info", "", bearerToken, nil)
	if withBearer != http.StatusOK {
		t.Fatalf("bearer %d", withBearer)
	}
	evil := unixStatus(t, path, http.MethodGet, "/v1/info", "http://evil.example", bearerToken, nil)
	if evil != http.StatusForbidden {
		t.Fatalf("evil origin %d", evil)
	}
	loopback := unixStatus(t, path, http.MethodGet, "/v1/info", "http://127.0.0.1:8765", bearerToken, nil)
	if loopback != http.StatusForbidden {
		t.Fatalf("implicit loopback origin %d", loopback)
	}

	login, err := http.NewRequest(http.MethodPost, "http://localhost/v1/login", strings.NewReader(`{"token":"`+bearerToken+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	login.Header.Set("Content-Type", "application/json")
	res, err := unixClient(path).Do(login)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || len(res.Cookies()) == 0 {
		t.Fatalf("login %d cookies %d", res.StatusCode, len(res.Cookies()))
	}
	cookie := res.Cookies()[0]
	cookieOnly := unixStatus(t, path, http.MethodGet, "/v1/info", "", "", cookie)
	if cookieOnly != http.StatusUnauthorized {
		t.Fatalf("cookie without origin %d", cookieOnly)
	}
	post := unixStatus(t, path, http.MethodPost, "/v1/logout", "", "", cookie)
	if post != http.StatusForbidden {
		t.Fatalf("cookie post without origin %d", post)
	}
	withOrigin := unixStatus(t, path, http.MethodGet, "/v1/info", origin, "", cookie)
	if withOrigin != http.StatusOK {
		t.Fatalf("cookie with public origin %d", withOrigin)
	}
}

func newUnixServer(t *testing.T, path, token string) *Server {
	t.Helper()
	srv, err := New(Config{
		Listen: "unix:" + path,
		Token:  token,
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func serveUnix(t *testing.T, srv *Server, path string) func() {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = srv.ListenAndServe()
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("unix", path, 50*time.Millisecond)
		if err == nil {
			c.Close()
			return func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Error("listener leaked")
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("socket did not accept")
	return func() {}
}

func serveUntilError(srv *Server) error {
	errc := make(chan error, 1)
	go func() {
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return err
	case <-time.After(2 * time.Second):
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return nil
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Fatalf("%s mode %o, want %o", filepath.Base(path), fi.Mode().Perm(), want)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		t.Fatalf("owner %#v", fi.Sys())
	}
}

func unixGet(t *testing.T, path, urlPath string, cookie *http.Cookie) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://localhost"+urlPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := unixClient(path).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s %d %s", urlPath, res.StatusCode, b)
	}
	return string(b)
}

func unixStatus(t *testing.T, path, method, urlPath, origin, bearer string, cookie *http.Cookie) int {
	t.Helper()
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(`{}`)
	}
	req, err := http.NewRequest(method, "http://localhost"+urlPath, body)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := unixClient(path).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func unixClient(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			},
		},
	}
}
