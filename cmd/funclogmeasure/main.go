// Command funclogmeasure reports which named functions and methods in the module
// have no type-resolved call to log/slog in the function body (see docs/OBSERVABILITY.md).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
)

type violation struct {
	Pkg      string `json:"pkg"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	FuncName string `json:"func"`
}

type report struct {
	FilesScanned     int         `json:"files_scanned"`
	FuncsConsidered  int         `json:"funcs_considered"`
	FuncsWithSlog    int         `json:"funcs_with_slog"`
	FuncsMissingSlog int         `json:"funcs_missing_slog"`
	Violations       []violation `json:"violations"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	root := flag.String("root", ".", "module root (directory containing go.mod)")
	tests := flag.Bool("tests", false, "include *_test.go files")
	includeTool := flag.Bool("include-tool", false, "include this command package (cmd/funclogmeasure)")
	enforce := flag.Bool("enforce", false, "exit with status 1 if any function lacks slog")
	maxPrint := flag.Int("max", 200, "max violation lines to print (0 = unlimited)")
	jsonOut := flag.Bool("json", false, "emit JSON summary and violations on stdout")
	flag.Parse()

	if err := run(*root, *tests, *includeTool, *enforce, *maxPrint, *jsonOut); err != nil {
		slog.Error("funclogmeasure failed", "operation", "funclogmeasure.run", "err", err)
		os.Exit(1)
	}
}

func run(root string, tests, includeTool, enforce bool, maxPrint int, jsonOut bool) error {
	modRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	slog.Debug("funclogmeasure scanning", "operation", "funclogmeasure.scan", "root", modRoot)

	rep, err := buildReport(modRoot, analyzeOpts{
		tests:       tests,
		includeTool: includeTool,
	})
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

	if enforce && rep.FuncsMissingSlog > 0 {
		return errors.New("one or more functions lack a type-resolved log/slog call")
	}
	return nil
}
