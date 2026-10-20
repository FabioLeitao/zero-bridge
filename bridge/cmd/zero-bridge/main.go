package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"zero-bridge/bridge/internal/acpclient"
	"zero-bridge/bridge/internal/auth"
	"zero-bridge/bridge/internal/observability"
	"zero-bridge/bridge/internal/server"
	"zero-bridge/bridge/internal/sessionmeta"
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
	listen         string
	workspace      string
	permTimeout    time.Duration
	tokenFile      string
	auditLog       string
	configPath     string
	limitFlags     map[string]string
	allowedHosts   []string
	publicOrigins  []string
	trustProxy     bool
	agent          acpclient.Command
	profile        string
	stateDir       string
	workspaceRoots []string

	// Observability (S16).
	otlpEndpoint string
	otelService  string
	otelSample   float64
}

func runArgs(args []string, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--healthcheck" {
		if len(args) != 2 {
			fmt.Fprintf(stderr, "zero-bridge: --healthcheck needs one URL\n")
			return 2
		}
		return healthcheck(args[1], stderr)
	}
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
	listen, err := server.ResolveListen(cfg.listen)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: %v\n", err)
		return 2
	}
	if listen.Network == "tcp" && token == "" {
		fmt.Fprintf(stderr, "zero-bridge: warning: no token on TCP %s; any local user who can open this port can call the API, including POST /a2a/message/send\n", listen.Addr)
	}
	logger := log.New(stderr, "", log.LstdFlags)
	// Observability (S16): set env for bridge/obs module.
	if cfg.otlpEndpoint != "" {
		if err := os.Setenv("OTLP_ENDPOINT", cfg.otlpEndpoint); err != nil {
			return 2
		}
		if cfg.otelService != "" {
			if err := os.Setenv("OTEL_SERVICE_NAME", cfg.otelService); err != nil {
				return 2
			}
		}
		// The sample ratio (cfg.otelSample) is not passed to the obs module through the environment here.
		// Start the OTel collector as a subprocess?
		// The obs module is a separate binary. We'll just set env.
		// The actual OTel initialization happens in bridge/obs/main if run separately.
		// For now, we set the global observer to noop since the sidecar is separate.
		observability.SetGlobal(observability.NoopObserver{})
	} else {
		observability.SetGlobal(observability.NoopObserver{})
	}
	stateDir := cfg.stateDir
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	meta := openSessionMeta(stateDir, logger)
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
		Listen:            cfg.listen,
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
		Meta:              meta,
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
	roots := cfg.workspaceRoots
	if len(roots) == 0 {
		roots = []string{srv.DefaultWorkspace()}
	}
	if err := srv.SetWorkspaceRoots(roots); err != nil {
		fmt.Fprintf(stderr, "zero-bridge: workspace root: %v\n", err)
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

func defaultStateDir() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, "zero-bridge")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "zero-bridge")
}

// openSessionMeta opens the label store. A failure is logged without the
// directory path and the bridge still starts. Meta routes then answer 503.
func openSessionMeta(dir string, logger *log.Logger) *sessionmeta.Store {
	if dir == "" {
		logger.Printf("session metadata store is not available")
		return nil
	}
	st, err := sessionmeta.Open(dir, func(msg string) {
		logger.Printf("%s", msg)
	})
	if err != nil {
		logger.Printf("session metadata store is not available")
		return nil
	}
	return st
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
		case "--workspace-root":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --workspace-root")
			}
			i++
			cfg.workspaceRoots = append(cfg.workspaceRoots, args[i])
		case "--state-dir":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --state-dir")
			}
			i++
			cfg.stateDir = args[i]
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
		case "--otlp-endpoint":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --otlp-endpoint")
			}
			i++
			cfg.otlpEndpoint = args[i]
		case "--otel-service":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --otel-service")
			}
			i++
			cfg.otelService = args[i]
		case "--otel-sample":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for --otel-sample")
			}
			i++
			var err error
			cfg.otelSample, err = parseFloat(args[i])
			if err != nil {
				return cfg, fmt.Errorf("otel sample: %v", err)
			}
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

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
