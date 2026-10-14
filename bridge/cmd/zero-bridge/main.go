package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"zero-bridge/bridge/internal/acpclient"
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
	return runArgs(os.Args[1:], os.Stderr)
}

type cliConfig struct {
	listen        string
	workspace     string
	permTimeout   time.Duration
	tokenFile     string
	auditLog      string
	configPath    string
	limitFlags    map[string]string
	allowedHosts  []string
	publicOrigins []string
	trustProxy    bool
	agent         acpclient.Command
	profile       string
}

func runArgs(args []string, stderr io.Writer) int {
	cfg, err := parseCLI(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	token, err := loadToken(cfg.tokenFile)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	var fileLimits map[string]string
	if cfg.configPath != "" {
		fileLimits, err = server.LoadLimitFile(cfg.configPath)
		if err != nil {
			fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
			return 2
		}
	}
	limits, err := server.ResolveLimits(cfg.limitFlags, limitEnv(), fileLimits)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	resolved, err := server.ResolveListen(cfg.listen)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	logger := log.New(stderr, "", log.LstdFlags)
	auditSink := auth.Sink(auth.NewLogSink(logger))
	if cfg.auditLog != "" {
		fileSink, err := auth.OpenFileSink(cfg.auditLog)
		if err != nil {
			fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
			return 2
		}
		auditSink = auth.NewMulti(auditSink, fileSink)
	}
	srv, err := server.New(server.Config{
		Listen:            resolved,
		AgentCmd:          cfg.agent.Path,
		AgentArgs:         cfg.agent.Args,
		Profile:           cfg.profile,
		Workspace:         cfg.workspace,
		PermissionTimeout: cfg.permTimeout,
		Logger:            logger,
		Audit:             auditSink,
		Token:             token,
		TokenFile:         cfg.tokenFile,
		AllowedHosts:      cfg.allowedHosts,
		PublicOrigins:     cfg.publicOrigins,
		TrustProxy:        cfg.trustProxy,
		SessionTTL:        limits.SessionTTL,
		SessionIdle:       limits.SessionIdle,
		MaxSessions:       limits.MaxSessions,
		AuthFailWindow:    limits.AuthFailWindow,
		AuthFailLimit:     limits.AuthFailLimit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	srv.ArmReload()
	if err := srv.PrepareWorkspace(); err != nil {
		fmt.Fprintf(stderr, "zero-bridge: workspace: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Boot(ctx); err != nil {
		fmt.Fprintf(stderr, "zero-bridge: boot: %v\n", err)
		return 1
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "zero-bridge: http: %v\n", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	return 0
}

func parseCLI(args []string, stderr io.Writer) (cliConfig, error) {
	cfg := cliConfig{
		listen:      "127.0.0.1:8765",
		permTimeout: 60 * time.Second,
		limitFlags:  map[string]string{},
		profile:     "zero",
	}
	var (
		agentCmd    string
		agentCmdSet bool
		agentArgs   []string
		agentArgSet bool
		zeroBin     string
		zeroBinSet  bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --listen")
			}
			i++
			cfg.listen = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --workspace")
			}
			i++
			cfg.workspace = args[i]
		case "--permission-timeout":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --permission-timeout")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return cfg, fmt.Errorf("permission timeout: %v", err)
			}
			cfg.permTimeout = d
		case "--trust-proxy":
			cfg.trustProxy = true
		case "--allowed-host":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --allowed-host")
			}
			i++
			cfg.allowedHosts = append(cfg.allowedHosts, args[i])
		case "--public-origin":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --public-origin")
			}
			i++
			cfg.publicOrigins = append(cfg.publicOrigins, args[i])
		case "--token-file":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --token-file")
			}
			i++
			cfg.tokenFile = args[i]
		case "--audit-log":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --audit-log")
			}
			i++
			cfg.auditLog = args[i]
		case "--config":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --config")
			}
			i++
			cfg.configPath = args[i]
		case "--session-ttl", "--session-idle", "--max-sessions", "--auth-fail-window", "--auth-fail-limit":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for %s", args[i])
			}
			key := strings.TrimPrefix(args[i], "--")
			i++
			cfg.limitFlags[key] = args[i]
		case "--agent-cmd":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --agent-cmd")
			}
			i++
			agentCmd = args[i]
			agentCmdSet = true
		case "--agent-arg":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --agent-arg")
			}
			i++
			agentArgs = append(agentArgs, args[i])
			agentArgSet = true
		case "--profile":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --profile")
			}
			i++
			cfg.profile = args[i]
		case "--zero-bin":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --zero-bin")
			}
			i++
			zeroBin = args[i]
			zeroBinSet = true
		}
	}
	switch cfg.profile {
	case "zero", "generic":
	default:
		return cfg, fmt.Errorf("unknown profile %q", cfg.profile)
	}
	if zeroBinSet && (agentCmdSet || agentArgSet) {
		return cfg, fmt.Errorf("--zero-bin cannot be combined with --agent-cmd or --agent-arg")
	}
	if zeroBinSet {
		fmt.Fprintln(stderr, "zero-bridge: --zero-bin is deprecated; use --agent-cmd and --agent-arg acp")
		agentCmd = zeroBin
		agentArgs = []string{"acp"}
	} else if !agentCmdSet {
		agentCmd = "zero"
		if !agentArgSet {
			agentArgs = []string{"acp"}
		}
	}
	resolved, err := acpclient.ResolveCommand(agentCmd, agentArgs)
	if err != nil {
		return cfg, err
	}
	cfg.agent = resolved
	return cfg, nil
}
