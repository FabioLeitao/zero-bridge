package acptest

import "os"

// MaybeMain exits with the fake agent when the test binary is invoked as `… acp` with ZB_FAKE_AGENT=1.
func MaybeMain() {
	if len(os.Args) > 1 && os.Args[1] == "acp" && os.Getenv("ZB_FAKE_AGENT") == "1" {
		Main()
		os.Exit(0)
	}
}
