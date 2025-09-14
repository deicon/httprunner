package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deicon/httprunner/parser"
	"github.com/deicon/httprunner/reporting"
	"github.com/deicon/httprunner/reporting/formatters/hierarchical"
	"github.com/deicon/httprunner/reporting/types"
	"github.com/deicon/httprunner/runner"
)

// version is populated at build time via ldflags (-X main.version=...)
var version = "dev"

func main() {
	// Command line flags
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.BoolVar(showVersion, "V", false, "Print version and exit")
	concurrency := flag.Int("u", 1, "Number of parallel virtual parallel users")
	iterations := flag.Int("i", 1, "Number of iterations")
	runtime := flag.Int("r", 0, "Runtime duration in seconds (0 means use iterations)")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")
	reportFormat := flag.String("report", "console", "Report format: console, html, csv, json")
	reportOutput := flag.String("output", "results", "Output directory for results and reports")
	reportDetail := flag.String("detail", "summary", "Report detail level: summary, goroutine, iteration, full")

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *requestFile == "" {
		fmt.Println("Error: -f flag is required")
		os.Exit(1)
	}

	// Validate runtime vs iterations parameters
	if *runtime < 0 {
		fmt.Println("Error: runtime (-r) must be 0 or positive")
		os.Exit(1)
	}
	if *iterations < 1 {
		fmt.Println("Error: iterations (-i) must be positive")
		os.Exit(1)
	}

	// Validate report format
	format := types.ReportFormat(strings.ToLower(*reportFormat))
	switch format {
	case types.FormatConsole, types.FormatHTML, types.FormatCSV, types.FormatJSON:
		// valid format
	default:
		fmt.Printf("Error: Invalid report format '%s'. Valid formats: console, html, csv, json\n", *reportFormat)
		os.Exit(1)
	}

	// Validate report detail level
	detailLevel := types.ReportDetailLevel(strings.ToLower(*reportDetail))
	switch detailLevel {
	case types.DetailSummary, types.DetailVirtualuser, types.DetailIteration, types.DetailFull:
		// valid detail level
	default:
		fmt.Printf("Error: Invalid report detail level '%s'. Valid levels: summary, goroutine, iteration, full\n", *reportDetail)
		os.Exit(1)
	}

	requests, err := parser.Parse(*requestFile)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	var r *runner.Runner

	// Always use streaming mode
	if *envFile != "" {
		r, err = runner.NewRunnerWithEnvFile(*concurrency, *iterations, *runtime, *delay, requests, *envFile, *reportOutput)
	} else {
		r, err = runner.NewRunner(*concurrency, *iterations, *runtime, *delay, requests, *reportOutput)
	}
	if err != nil {
		fmt.Printf("Error creating streaming runner: %v\n", err)
		os.Exit(1)
	}

	// Generate appropriate report based on detail level
	var reportContent string

	if detailLevel == types.DetailSummary {
		report, streamErr := r.Run()
		if streamErr != nil {
			fmt.Printf("Error running execution: %v\n", streamErr)
			os.Exit(1)
		}
		formatter := reporting.GetFormatter(format)
		reportContent, err = formatter.Format(report)
	} else {
		hierarchicalReport, streamErr := r.RunHierarchical()
		if streamErr != nil {
			fmt.Printf("Error running hierarchical execution: %v\n", streamErr)
			os.Exit(1)
		}
		hierarchicalFormatter := &hierarchical.HierarchicalFormatter{
			DetailLevel: detailLevel,
			Format:      format,
		}
		reportContent, err = hierarchicalFormatter.FormatHierarchical(hierarchicalReport)
	}

	if err != nil {
		fmt.Printf("Error formatting report: %v\n", err)
		os.Exit(1)
	}

	// Output report - always save to file and print to stdout for console format
	if format == types.FormatConsole {
		// For console format, print to stdout
		fmt.Print(reportContent)
	} else {
		// For other formats, save to file
		filename := filepath.Join(*reportOutput, "report")
		switch format {
		case types.FormatHTML:
			filename += ".html"
		case types.FormatCSV:
			filename += ".csv"
		case types.FormatJSON:
			filename += ".json"
		default:
			filename += ".txt"
		}

		if err := os.WriteFile(filename, []byte(reportContent), 0644); err != nil {
			fmt.Printf("Error writing report to file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nFormatted report saved to: %s\n", filename)
	}

	fmt.Printf("Raw results available in: %s/raw-results-*.jsonl\n", *reportOutput)
}
