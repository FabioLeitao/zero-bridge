package server

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultSessionTTL is the absolute life of a login cookie.
	DefaultSessionTTL = 12 * time.Hour
	// MaxSessionTTL is the operator cap. Thirty days is long enough for a
	// workstation that stays up, and short enough that a stolen cookie dies
	// without an operator action.
	MaxSessionTTL = 30 * 24 * time.Hour
	// DefaultSessionIdle is the sliding quiet period. It is shorter than the
	// TTL so an abandoned browser does not keep a cookie until the absolute end.
	DefaultSessionIdle = 2 * time.Hour
	// DefaultMaxSessions is one person with a few browsers. MaxMaxSessions
	// bounds the in-memory map: past 64 the bridge is no longer a single-operator tool.
	DefaultMaxSessions = 8
	MaxMaxSessions     = 64
	// DefaultAuthFailWindow is the shared login and Bearer failure window.
	// The floor is one second because Retry-After is a whole number of seconds.
	// The cap is one hour so a mistake is not a day-long lockout.
	DefaultAuthFailWindow = time.Minute
	MinAuthFailWindow     = time.Second
	MaxAuthFailWindow     = time.Hour
	// DefaultAuthFailLimit counts failures inside the window. One is the
	// strictest useful policy. One hundred bounds the timestamp list for a
	// shared address.
	DefaultAuthFailLimit = 5
	MaxAuthFailLimit     = 100
)

// Limits is the finite set of auth bounds. Zero values are not valid results.
type Limits struct {
	SessionTTL     time.Duration
	SessionIdle    time.Duration
	MaxSessions    int
	AuthFailWindow time.Duration
	AuthFailLimit  int
}

// LoadLimitFile reads key/value lines. Empty lines and lines starting with #
// are ignored. An unknown key is an error.
func LoadLimitFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, " ")
		if !ok {
			key, val, ok = strings.Cut(line, "\t")
		}
		if !ok || strings.TrimSpace(val) == "" {
			return nil, fmt.Errorf("config line %d: want key and value", lineNo)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if !knownLimitKey(key) {
			return nil, fmt.Errorf("config line %d: unknown key %q", lineNo, key)
		}
		out[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return out, nil
}

func knownLimitKey(key string) bool {
	switch key {
	case "session-ttl", "session-idle", "max-sessions", "auth-fail-window", "auth-fail-limit":
		return true
	default:
		return false
	}
}

// ResolveLimits applies flag, then environment, then the config file, then
// the defaults. An explicit 0, a negative value, or "unlimited" is rejected.
func ResolveLimits(flag, env, file map[string]string) (Limits, error) {
	var out Limits
	var err error
	ttlRaw, ttlSet := pickLimit("session-ttl", flag, env, file)
	if !ttlSet {
		out.SessionTTL = DefaultSessionTTL
	} else if out.SessionTTL, err = parseFiniteDuration("session-ttl", ttlRaw, time.Nanosecond, MaxSessionTTL); err != nil {
		return Limits{}, err
	}
	idleRaw, idleSet := pickLimit("session-idle", flag, env, file)
	if !idleSet {
		out.SessionIdle = DefaultSessionIdle
	} else if out.SessionIdle, err = parseFiniteDuration("session-idle", idleRaw, time.Nanosecond, MaxSessionTTL); err != nil {
		return Limits{}, err
	}
	if out.SessionIdle > out.SessionTTL {
		return Limits{}, fmt.Errorf("session-idle %s is longer than session-ttl %s", out.SessionIdle, out.SessionTTL)
	}
	maxRaw, maxSet := pickLimit("max-sessions", flag, env, file)
	if !maxSet {
		out.MaxSessions = DefaultMaxSessions
	} else if out.MaxSessions, err = parsePositiveInt("max-sessions", maxRaw, MaxMaxSessions); err != nil {
		return Limits{}, err
	}
	winRaw, winSet := pickLimit("auth-fail-window", flag, env, file)
	if !winSet {
		out.AuthFailWindow = DefaultAuthFailWindow
	} else if out.AuthFailWindow, err = parseFiniteDuration("auth-fail-window", winRaw, MinAuthFailWindow, MaxAuthFailWindow); err != nil {
		return Limits{}, err
	}
	limRaw, limSet := pickLimit("auth-fail-limit", flag, env, file)
	if !limSet {
		out.AuthFailLimit = DefaultAuthFailLimit
	} else if out.AuthFailLimit, err = parsePositiveInt("auth-fail-limit", limRaw, MaxAuthFailLimit); err != nil {
		return Limits{}, err
	}
	return out, nil
}

func pickLimit(key string, sources ...map[string]string) (string, bool) {
	for _, src := range sources {
		if src == nil {
			continue
		}
		if v, ok := src[key]; ok {
			return v, true
		}
	}
	return "", false
}

func parseFiniteDuration(name, raw string, min, max time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "unlimited") {
		return 0, fmt.Errorf("%s: unlimited is not a limit", name)
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		n, nerr := strconv.ParseInt(raw, 10, 64)
		if nerr != nil {
			return 0, fmt.Errorf("%s: %w", name, err)
		}
		d = time.Duration(n) * time.Second
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: must be greater than zero", name)
	}
	if d < min {
		return 0, fmt.Errorf("%s: %s is below the floor %s", name, d, min)
	}
	if d > max {
		return 0, fmt.Errorf("%s: %s exceeds the cap %s", name, d, max)
	}
	return d, nil
}

func parsePositiveInt(name, raw string, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "unlimited") {
		return 0, fmt.Errorf("%s: unlimited is not a limit", name)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s: must be greater than zero", name)
	}
	if n > max {
		return 0, fmt.Errorf("%s: %d exceeds the cap %d", name, n, max)
	}
	return n, nil
}

func applyLimitDefaults(cfg *Config) error {
	lim, err := ResolveLimits(map[string]string{}, nil, nil)
	if err != nil {
		return err
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = lim.SessionTTL
	}
	if cfg.SessionIdle == 0 {
		cfg.SessionIdle = lim.SessionIdle
		if cfg.SessionIdle > cfg.SessionTTL {
			cfg.SessionIdle = cfg.SessionTTL
		}
	}
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = lim.MaxSessions
	}
	if cfg.AuthFailWindow == 0 {
		cfg.AuthFailWindow = lim.AuthFailWindow
	}
	if cfg.AuthFailLimit == 0 {
		cfg.AuthFailLimit = lim.AuthFailLimit
	}
	if cfg.SessionTTL <= 0 || cfg.SessionTTL > MaxSessionTTL {
		return fmt.Errorf("session-ttl must be greater than zero and at most %s", MaxSessionTTL)
	}
	if cfg.SessionIdle <= 0 || cfg.SessionIdle > cfg.SessionTTL {
		return fmt.Errorf("session-idle must be greater than zero and no longer than session-ttl")
	}
	if cfg.MaxSessions <= 0 || cfg.MaxSessions > MaxMaxSessions {
		return fmt.Errorf("max-sessions must be from 1 to %d", MaxMaxSessions)
	}
	if cfg.AuthFailWindow < MinAuthFailWindow || cfg.AuthFailWindow > MaxAuthFailWindow {
		return fmt.Errorf("auth-fail-window must be from %s to %s", MinAuthFailWindow, MaxAuthFailWindow)
	}
	if cfg.AuthFailLimit <= 0 || cfg.AuthFailLimit > MaxAuthFailLimit {
		return fmt.Errorf("auth-fail-limit must be from 1 to %d", MaxAuthFailLimit)
	}
	return nil
}
