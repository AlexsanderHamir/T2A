package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type violation struct {
	Pkg      string `json:"pkg"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	FuncName string `json:"func"`
}

type satisfactionCounts struct {
	DirectSlog     int `json:"direct_slog"`
	TraceDelegate  int `json:"trace_delegate"`
	AutoExempt     int `json:"auto_exempt"`
	Directive      int `json:"directive"`
}

type report struct {
	FilesScanned      int                `json:"files_scanned"`
	FuncsConsidered   int                `json:"funcs_considered"`
	FuncsSatisfied    int                `json:"funcs_satisfied"`
	FuncsMissingTrace int                `json:"funcs_missing_trace"`
	Satisfaction      satisfactionCounts `json:"satisfaction"`
	Violations        []violation        `json:"violations"`
}

func (r *report) recordSatisfied(layer string) {
	r.FuncsSatisfied++
	switch layer {
	case layerDirectSlog:
		r.Satisfaction.DirectSlog++
	case layerTraceDelegate:
		r.Satisfaction.TraceDelegate++
	case layerAutoExempt:
		r.Satisfaction.AutoExempt++
	case layerDirective:
		r.Satisfaction.Directive++
	}
}

func printTextReport(rep *report, maxPrint int, modRoot string) {
	var pct float64
	if rep.FuncsConsidered > 0 {
		pct = 100.0 * float64(rep.FuncsSatisfied) / float64(rep.FuncsConsidered)
	}
	fmt.Fprintf(os.Stdout, "funclogmeasure: files=%d funcs=%d satisfied=%d missing_trace=%d (%.1f%% satisfy trace contract)\n",
		rep.FilesScanned, rep.FuncsConsidered, rep.FuncsSatisfied, rep.FuncsMissingTrace, pct)
	fmt.Fprintf(os.Stdout, "  satisfaction: direct_slog=%d trace_delegate=%d auto_exempt=%d directive=%d\n",
		rep.Satisfaction.DirectSlog, rep.Satisfaction.TraceDelegate,
		rep.Satisfaction.AutoExempt, rep.Satisfaction.Directive)
	if rep.FuncsMissingTrace == 0 {
		fmt.Fprintln(os.Stdout, "All considered functions satisfy the trace-line contract.")
		return
	}
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Functions missing trace coverage (no slog, delegate, auto-exempt, or //funclogmeasure:skip):")
	n := 0
	for _, v := range rep.Violations {
		rel, _ := filepath.Rel(modRoot, v.File)
		if rel == "" {
			rel = v.File
		}
		fmt.Fprintf(os.Stdout, "%s:%d\t%s\t%s\n", rel, v.Line, v.Pkg, v.FuncName)
		n++
		if maxPrint > 0 && n >= maxPrint {
			rest := len(rep.Violations) - n
			if rest > 0 {
				fmt.Fprintf(os.Stdout, "... and %d more (increase -max or use -json)\n", rest)
			}
			break
		}
	}
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Direct slog counts type-resolved log/slog calls. Nested func literals do not count for the outer func.")
}
