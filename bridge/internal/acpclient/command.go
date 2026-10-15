package acpclient

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Command is the agent process to start. Args are separate argv elements.
// Nothing here is passed through a shell.
type Command struct {
	Path string
	Args []string
}

// ResolveCommand checks bin and keeps args as individual argv elements.
// An absolute bin must name an existing executable file. Any other bin is
// resolved with LookPath. An empty bin is refused.
func ResolveCommand(bin string, args []string) (Command, error) {
	if bin == "" {
		return Command{}, fmt.Errorf("agent command is empty")
	}
	path, err := resolveBin(bin)
	if err != nil {
		return Command{}, err
	}
	out := make([]string, len(args))
	copy(out, args)
	return Command{Path: path, Args: out}, nil
}

func resolveBin(bin string) (string, error) {
	if filepath.IsAbs(bin) {
		st, err := os.Stat(bin)
		if err != nil {
			return "", fmt.Errorf("agent command %q: %w", bin, err)
		}
		if st.IsDir() {
			return "", fmt.Errorf("agent command %q is a directory", bin)
		}
		if st.Mode()&0o111 == 0 {
			return "", fmt.Errorf("agent command %q is not executable", bin)
		}
		return bin, nil
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("agent command %q: %w", bin, err)
	}
	return path, nil
}
