package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"genFu/internal/eval"
)

func main() {
	scenariosPath := flag.String("scenarios", "", "path to benchmark scenarios json")
	predictionsPath := flag.String("predictions", "", "path to system predictions json")
	format := flag.String("format", "markdown", "output format: markdown or json")
	flag.Parse()

	if *scenariosPath == "" || *predictionsPath == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/eval-report -scenarios <file> -predictions <file> [-format markdown|json]")
		os.Exit(2)
	}

	scenarios, predictions, err := eval.LoadBenchmarkInputs(*scenariosPath, *predictionsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load inputs: %v\n", err)
		os.Exit(1)
	}
	report, err := eval.BuildReport(scenarios, predictions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build report: %v\n", err)
		os.Exit(1)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Print(eval.RenderMarkdownSummary(report))
	}
}
