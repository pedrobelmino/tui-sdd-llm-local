package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check Ollama, GPU, model, and .specs/ health",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		ok := true

		fmt.Fprintln(os.Stderr, "tsll doctor")
		fmt.Fprintln(os.Stderr, stringsRepeat("─", 40))

		client := ollama.NewClient(cfg.OllamaHost)
		if client.Reachable(context.Background()) {
			printOK("Ollama reachable at " + cfg.OllamaHost)
			snap := ollama.FetchSnapshot(context.Background(), client, cfg.Model)
			if snap.ModelMissing {
				fmt.Fprintf(os.Stderr, "✗ model %s not pulled — run: ollama pull %s\n", cfg.Model, cfg.Model)
				ok = false
			} else {
				printOK("model " + cfg.Model + " available")
			}
		} else {
			fmt.Fprintf(os.Stderr, "✗ Ollama unreachable at %s\n", cfg.OllamaHost)
			ok = false
		}

		gpuSnap, _ := gpu.Query(context.Background())
		if gpuSnap.Available && len(gpuSnap.Devices) > 0 {
			d := gpuSnap.Devices[0]
			printOK(fmt.Sprintf("GPU [%s] %s — util %.0f%% VRAM %d/%d MiB",
				gpuSnap.Vendor, d.Name, d.Utilization, d.MemoryUsed, d.MemoryTotal))
		} else {
			fmt.Fprintf(os.Stderr, "⚠ GPU metrics unavailable (%s)\n", gpuSnap.Error)
		}

		if ctx, err := project.RequireRoot(); err == nil {
			printOK(".specs/project at " + ctx.Root)
		} else {
			fmt.Fprintln(os.Stderr, "⚠ not inside a tsll project (tsll init)")
		}

		fmt.Fprintf(os.Stderr, "\nConfig: model=%s gpu_prefer=%s\n", cfg.Model, cfg.GPUPrefer)
		if !ok {
			return fmt.Errorf("doctor found issues")
		}
		return nil
	},
}

func stringsRepeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
