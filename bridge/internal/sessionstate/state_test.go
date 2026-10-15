package sessionstate

import "testing"

func TestTransitionTable(t *testing.T) {
	cases := []struct {
		name  string
		level string
		steps []Event
		want  string
	}{
		{name: "fresh", level: LevelFull, want: Idle},
		{name: "prompt", level: LevelFull, steps: []Event{{Kind: KindPrompt}}, want: Working},
		{name: "end turn", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "end_turn"}}, want: Done},
		{name: "max tokens", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "max_tokens"}}, want: Done},
		{name: "refusal", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "refusal"}}, want: Done},
		{name: "other reason", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "custom"}}, want: Done},
		{name: "cancelled", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "cancelled"}}, want: Idle},
		{name: "empty stop", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindStop}}, want: Idle},
		{name: "turn error", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindFail}}, want: Error},
		{name: "agent exit", level: LevelFull, steps: []Event{{Kind: KindPrompt}, {Kind: KindExit}}, want: Error},
		{name: "permission alone", level: LevelFull, steps: []Event{{Kind: KindPermit}}, want: Blocked},
		{
			name:  "permission during prompt",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}},
			want:  Blocked,
		},
		{
			name:  "release while prompt continues",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}, {Kind: KindRelease}},
			want:  Working,
		},
		{
			name:  "stop while permission still open",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}, {Kind: KindStop, Stop: "end_turn"}},
			want:  Blocked,
		},
		{
			name:  "release after end turn",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}, {Kind: KindStop, Stop: "end_turn"}, {Kind: KindRelease}},
			want:  Done,
		},
		{
			name:  "release after cancel",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}, {Kind: KindStop, Stop: "cancelled"}, {Kind: KindRelease}},
			want:  Idle,
		},
		{
			name:  "exit while blocked",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}, {Kind: KindExit}},
			want:  Blocked,
		},
		{
			name:  "new prompt clears turn error",
			level: LevelFull,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindFail}, {Kind: KindPrompt}},
			want:  Working,
		},
		{
			name:  "exit is not cleared by a prompt",
			level: LevelFull,
			steps: []Event{{Kind: KindExit}, {Kind: KindPrompt}},
			want:  Error,
		},
		{name: "basic prompt", level: LevelBasic, steps: []Event{{Kind: KindPrompt}}, want: Working},
		{name: "basic idle", level: LevelBasic, want: Idle},
		{
			name:  "basic never blocked",
			level: LevelBasic,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindPermit}},
			want:  Working,
		},
		{
			name:  "basic permission without prompt is idle",
			level: LevelBasic,
			steps: []Event{{Kind: KindPermit}},
			want:  Idle,
		},
		{
			name:  "basic hides done and error",
			level: LevelBasic,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindStop, Stop: "end_turn"}},
			want:  Idle,
		},
		{
			name:  "basic hides turn error",
			level: LevelBasic,
			steps: []Event{{Kind: KindPrompt}, {Kind: KindFail}},
			want:  Idle,
		},
		{name: "unknown level is basic", level: "other", steps: []Event{{Kind: KindPermit}}, want: Idle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.level)
			for _, ev := range tc.steps {
				m = m.Apply(ev)
			}
			if got := m.State(); got != tc.want {
				t.Fatalf("state %s, want %s", got, tc.want)
			}
		})
	}
}

func TestApplyDoesNotMutateReceiver(t *testing.T) {
	m := New(LevelFull)
	_ = m.Apply(Event{Kind: KindPrompt})
	if m.State() != Idle {
		t.Fatalf("receiver changed to %s", m.State())
	}
}

func TestReleaseDoesNotGoNegative(t *testing.T) {
	m := New(LevelFull).Apply(Event{Kind: KindRelease})
	if m.State() != Idle {
		t.Fatalf("state %s", m.State())
	}
}
