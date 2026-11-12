// prompts prints what this build's A (guide) or S (feedback) key copies.
// Usage: go run ./scripts/guide-eval/prompts guide|feedback
package main

import (
	"fmt"
	"os"

	"laziergit/internal/reviewbridge"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "feedback" {
		fmt.Println(reviewbridge.FeedbackPrompt)
		return
	}
	fmt.Println(reviewbridge.Prompt)
}
