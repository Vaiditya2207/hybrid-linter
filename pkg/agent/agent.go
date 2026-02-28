package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Vaiditya2207/hybrid-linter/pkg/engine"
)

// TerminalAgent manages the interactive REPL chat session.
type TerminalAgent struct {
	engine *engine.Engine
}

func NewTerminalAgent(e *engine.Engine) *TerminalAgent {
	return &TerminalAgent{engine: e}
}

// ChatLoop starts an interactive session about a specific code vulnerability.
func (a *TerminalAgent) ChatLoop(ctx context.Context, fileContext string, bugDescription string) (string, bool) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n\033[1;36m🤖 Agent Interactive Fixer [Mode: Local LLM]\033[0m\n")
	fmt.Printf("\033[33mVulnerability:\033[0m %s\n", bugDescription)
	fmt.Printf("Type \033[32m/apply\033[0m to accept fix, \033[31m/skip\033[0m to ignore, or just chat!\n\n")

	for {
		fmt.Printf("\033[1;36mAgent > \033[0m")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "/apply" {
			return "", true
		}
		if input == "/skip" {
			return "", false
		}
		if input == "" {
			continue
		}

		// Chat with LLM
		prompt := fmt.Sprintf("Context:\n%s\n\nIssue: %s\n\nUser Question: %s\n\nResponse:", 
			fileContext, bugDescription, input)
		
		fmt.Printf("\033[37mThinking...\033[0m\r")
		resp, err := a.engine.Predict(ctx, prompt, 512)
		if err != nil {
			fmt.Printf("\033[31mError: %v\033[0m\n", err)
			continue
		}

		fmt.Printf("\033[32m%s\033[0m\n\n", strings.TrimSpace(resp))
	}
}
