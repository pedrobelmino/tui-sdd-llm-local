package cmd

import (
	"fmt"
	"os"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	initName     string
	initVision   string
	initNonInter bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a tsll project (.specs/project/)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		ctx, _ := project.FindProject(cwd)
		if ctx.Valid && !initForce {
			return fmt.Errorf("project already initialized at %s (use --force)", ctx.Root)
		}

		name := initName
		vision := initVision
		target := "dev solo"
		solves := "structured spec-driven development with local AI"
		stack := "- Language: Go\n- LLM: Ollama (qwen2.5-coder)"
		scopeIn := "- tsll workflow: init, specify, tasks, run\n- TUI dashboard"
		scopeOut := "- Cloud APIs\n- macOS/Windows"

		if !initNonInter {
			if name == "" {
				name = promptLine("Project name")
			}
			if vision == "" {
				vision = promptLine("Vision (1-2 sentences)")
			}
			target = promptLine("Target users")
			solves = promptLine("Problem solved")
			stack = promptMultiline("Tech stack (bullet lines)")
			scopeIn = promptMultiline("v1 scope (bullet lines)")
			scopeOut = promptMultiline("Out of scope (bullet lines)")
		}

		if err := workflow.InitProject(workflow.InitParams{
			Root: cwd, Name: name, Vision: vision,
			Target: target, Solves: solves, Stack: stack,
			ScopeIn: scopeIn, ScopeOut: scopeOut, Force: initForce,
		}); err != nil {
			return err
		}

		for _, rel := range []string{
			".specs/project/PROJECT.md",
			".specs/project/ROADMAP.md",
			".specs/project/STATE.md",
		} {
			printOK("created " + rel)
		}

		fmt.Fprintf(os.Stderr, "\nNext: press 2 in TUI → n for new feature\n")
		return nil
	},
}

var initForce bool

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "project name")
	initCmd.Flags().StringVar(&initVision, "vision", "", "project vision")
	initCmd.Flags().BoolVar(&initNonInter, "yes", false, "non-interactive (use flags/defaults)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing project files")
	rootCmd.AddCommand(initCmd)
}

