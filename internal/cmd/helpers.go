package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/config"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
)

func loadCfg() config.Config {
	return config.Load()
}

func newGenClient(cfg config.Config) ollama.GenerateClient {
	return ollama.NewGenerateClient(cfg.OllamaHost)
}

func promptLine(label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptMultiline(label string) string {
	fmt.Fprintf(os.Stderr, "%s (end with empty line):\n", label)
	var lines []string
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func printOK(msg string) {
	fmt.Fprintf(os.Stderr, "✓ %s\n", msg)
}
