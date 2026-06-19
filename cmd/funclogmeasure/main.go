// Command funclogmeasure reports which named functions and methods in the module
// lack trace-line coverage (direct slog, trace delegate, structural auto-exempt,
// or //funclogmeasure:skip). See docs/domain/observability-trace-lines.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	root := flag.String("root", ".", "module root (directory containing go.mod)")
	tests := flag.Bool("tests", false, "include *_test.go files")
	includeTool := flag.Bool("include-tool", false, "include this command package (cmd/funclogmeasure)")
	enforce := flag.Bool("enforce", false, "exit with status 1 if any function lacks trace coverage")
	injectDirectives := flag.Bool("inject-directives", false, "one-shot: add //funclogmeasure:skip for all current violations, then exit")
	maxPrint := flag.Int("max", 200, "max violation lines to print (0 = unlimited)")
	jsonOut := flag.Bool("json", false, "emit JSON summary and violations on stdout")
	flag.Parse()

	if err := run(*root, *tests, *includeTool, *enforce, *injectDirectives, *maxPrint, *jsonOut); err != nil {
		slog.Error("funclogmeasure failed", "operation", "funclogmeasure.run", "err", err)
		os.Exit(1)
	}
}

func run(root string, tests, includeTool, enforce, injectDirectives bool, maxPrint int, jsonOut bool) error {
	modRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	opts := analyzeOpts{tests: tests, includeTool: includeTool}

	if injectDirectives {
		n, err := injectDirectivesForViolations(modRoot, opts)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "funclogmeasure: injected %d //funclogmeasure:skip directives\n", n)
		return nil
	}

	slog.Debug("funclogmeasure scanning", "operation", "funclogmeasure.scan", "root", modRoot)

	rep, err := buildReport(modRoot, opts)
	if err != nil {
		return err
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		printTextReport(rep, maxPrint, modRoot)
	}

	if enforce && rep.FuncsMissingTrace > 0 {
		return errors.New("one or more functions lack trace-line coverage")
	}
	return nil
}
