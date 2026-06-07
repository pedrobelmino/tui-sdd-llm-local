package main

import (
	"os"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
