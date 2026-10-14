package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"zero-bridge/bridge/internal/auth"
	"zero-bridge/bridge/internal/server"
)

func limitEnv() map[string]string {
	out := map[string]string{}
	for _, pair := range []struct{ key, env string }{
		{"session-ttl", "ZERO_BRIDGE_SESSION_TTL"},
		{"session-idle", "ZERO_BRIDGE_SESSION_IDLE"},
		{"max-sessions", "ZERO_BRIDGE_MAX_SESSIONS"},
		{"auth-fail-window", "ZERO_BRIDGE_AUTH_FAIL_WINDOW"},
		{"auth-fail-limit", "ZERO_BRIDGE_AUTH_FAIL_LIMIT"},
	} {
		if v, ok := os.LookupEnv(pair.env); ok {
			out[pair.key] = v
		}
	}
	return out
}

func loadToken(tokenFile string) (string, error) {
	envToken := os.Getenv("ZERO_BRIDGE_TOKEN")
	if tokenFile != "" && envToken != "" {
		return "", fmt.Errorf("set only one of --token-file and ZERO_BRIDGE_TOKEN")
	}
	if tokenFile != "" {
		return auth.LoadFile(tokenFile)
	}
	if envToken == "" {
		return "", nil
	}
	if err := auth.Validate(envToken); err != nil {
		return "", err
	}
	return envToken, nil
}

func main() {
	os.Exit(run())
}

func run() int {
	listen := "127.0.0.1:8765"
	zeroBin := "zero"
	workspace := ""
	permTimeout := 60 * time.Second
	tokenFile := ""
	auditLog := ""
	configPath := ""
	limitFlags := map[string]string{}
	var allowedHosts []string
	var publicOrigins []string
	trustProxy := false
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--listen":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --listen")
				return 2
			}
			i++
			listen = os.Args[i]
		case "--workspace":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --workspace")
				return 2
			}
			i++
			workspace = os.Args[i]
		case "--permission-timeout":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --permission-timeout")
				return 2
			}
			i++
			d, err := time.ParseDuration(os.Args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "zero-bridge: permission timeout: %v\n", err)
				return 2
			}
			permTimeout = d
		case "--trust-proxy":
			trustProxy = true
		case "--allowed-host":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --allowed-host")
				return 2
			}
			i++
			allowedHosts = append(allowedHosts, os.Args[i])
		case "--public-origin":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --public-origin")
				return 2
			}
			i++
			publicOrigins = append(publicOrigins, os.Args[i])
		case "--token-file":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --token-file")
				return 2
			}
			i++
			tokenFile = os.Args[i]
		case "--audit-log":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --audit-log")
				return 2
			}
			i++
			auditLog = os.Args[i]
		case "--config":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --config")
				return 2
			}
			i++
			configPath = os.Args[i]
		case "--session-ttl", "--session-idle", "--max-sessions", "--auth-fail-window", "--auth-fail-limit":
			if i+1 >= len(os.Args) {
				fmt.Fprintf(os.Stderr, "missing value for %s\n", os.Args[i])
				return 2
			}
			key := strings.TrimPrefix(os.Args[i], "--")
			i++
			limitFlags[key] = os.Args[i]
		case "--zero-bin":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing value for --zero-bin")
				return 2
			}
			i++
			zeroBin = os.Args[i]
		}
	}
	token, err := loadToken(tokenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
		return 2
	}
	var fileLimits map[string]string
	if configPath != "" {
		fileLimits, err = server.LoadLimitFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
			return 2
		}
	}
	limits, err := server.ResolveLimits(limitFlags, limitEnv(), fileLimits)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
		return 2
	}
	resolved, err := server.ResolveListen(listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
		return 2
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	auditSink := auth.Sink(auth.NewLogSink(logger))
	if auditLog != "" {
		fileSink, err := auth.OpenFileSink(auditLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
			return 2
		}
		auditSink = auth.NewMulti(auditSink, fileSink)
	}
	srv, err := server.New(server.Config{
		Listen:            resolved,
		ZeroBin:           zeroBin,
		Workspace:         workspace,
		PermissionTimeout: permTimeout,
		Logger:            logger,
		Audit:             auditSink,
		Token:             token,
		AllowedHosts:      allowedHosts,
		PublicOrigins:     publicOrigins,
		TrustProxy:        trustProxy,
		SessionTTL:        limits.SessionTTL,
		SessionIdle:       limits.SessionIdle,
		MaxSessions:       limits.MaxSessions,
		AuthFailWindow:    limits.AuthFailWindow,
		AuthFailLimit:     limits.AuthFailLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: %v\n", err)
		return 2
	}
	if err := srv.PrepareWorkspace(); err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: workspace: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Boot(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "zero-bridge: boot: %v\n", err)
		return 1
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "zero-bridge: http: %v\n", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	return 0
}
