// Package sessionstate derives one session label from ACP traffic the
// bridge already sees. It has no I/O. It never stores a prompt, a tool
// argument, or a secret.
//
// Full level uses the priority blocked, then error, then working, then
// done, then idle. A pending permission is blocked even while a prompt
// is in flight. Basic level is only working or idle, and never blocked,
// done, or error.
//
// After the prompt returns, the resting label depends on the stop reason:
//
//	end_turn, max_tokens, max_turn_requests, refusal, any other non-empty reason  done
//	cancelled, or an empty reason                                                   idle
//	the prompt call failed                                                          error
//	the agent process exited                                                        error
//
// Cancel is idle because the turn did not finish. Any other reported
// reason is done because the turn ended. A later prompt clears a turn
// error. An agent exit stays until the process is replaced.
package sessionstate

const (
	// LevelFull reports the whole vocabulary.
	LevelFull = "full"
	// LevelBasic reports only working and idle.
	LevelBasic = "basic"

	Working = "working"
	Blocked = "blocked"
	Done    = "done"
	Error   = "error"
	Idle    = "idle"

	// KindPrompt marks a prompt call in flight.
	KindPrompt = "prompt"
	// KindPermit marks one permission request waiting on the user.
	KindPermit = "permit"
	// KindRelease marks one waiting permission cleared.
	KindRelease = "release"
	// KindStop marks a prompt result that carried a stop reason.
	KindStop = "stop"
	// KindFail marks a prompt call that failed.
	KindFail = "fail"
	// KindExit marks the agent process leaving.
	KindExit = "exit"
)

// Event is one transition. Stop is read only for KindStop.
type Event struct {
	Kind string
	Stop string
}

// Machine is the pure state of one session.
type Machine struct {
	level   string
	prompt  bool
	pending int
	rest    string
	down    bool
}

// New starts at idle. An unknown level is basic, so a generic agent
// cannot be reported as blocked.
func New(level string) Machine {
	if level != LevelFull {
		level = LevelBasic
	}
	return Machine{level: level}
}

// Apply returns the machine after ev. The receiver is unchanged.
func (m Machine) Apply(ev Event) Machine {
	switch ev.Kind {
	case KindPrompt:
		m.prompt = true
		if m.rest == Error {
			m.rest = ""
		}
	case KindPermit:
		m.pending++
	case KindRelease:
		if m.pending > 0 {
			m.pending--
		}
	case KindStop:
		m.prompt = false
		m.rest = RestAfterStop(ev.Stop)
	case KindFail:
		m.prompt = false
		m.rest = Error
	case KindExit:
		m.prompt = false
		m.down = true
	}
	return m
}

// State is the label a client may show.
func (m Machine) State() string {
	if m.level != LevelFull {
		if m.prompt {
			return Working
		}
		return Idle
	}
	if m.pending > 0 {
		return Blocked
	}
	if m.down || m.rest == Error {
		return Error
	}
	if m.prompt {
		return Working
	}
	if m.rest == Done {
		return Done
	}
	return Idle
}

// RestAfterStop maps a stop reason to the resting label.
// cancelled and an empty reason are idle. Every other reason is done.
func RestAfterStop(reason string) string {
	if reason == "" || reason == "cancelled" {
		return Idle
	}
	return Done
}
