// prompts prints what this build's A (guide) or S (feedback) key copies.
// Usage: go run ./scripts/guide-eval/prompts guide|feedback [server command]
package main

import (
	"fmt"
	"os"

	"laziergit/internal/reviewbridge"
)

func main() {
	server := "laziergit --review-mcp ."
	if len(os.Args) == 3 {
		server = os.Args[2]
	}
	if len(os.Args) >= 2 && os.Args[1] == "feedback" {
		fmt.Println(reviewbridge.FeedbackPrompt(server))
		return
	}
	fmt.Println(reviewbridge.Prompt(server))
}
