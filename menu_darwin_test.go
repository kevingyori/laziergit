package main

import (
	"os/exec"
	"testing"
)

func TestAppleScriptString(t *testing.T) {
	for _, value := range []string{
		"/Applications/laziergit.app/Contents/MacOS/laziergit",
		`/Applications/Bob's "app" \\ $HOME $(exit 1) ` + "`exit 1`",
		"path\nwith\rnewlines",
	} {
		output, err := exec.Command("/usr/bin/osascript", "-e", "return "+appleScriptString(value)).CombinedOutput()
		if err != nil {
			t.Fatalf("AppleScript failed: %v: %s", err, output)
		}
		if string(output) != value+"\n" {
			t.Fatalf("AppleScript string round trip: got %q, want %q", output, value+"\n")
		}
	}
}
