package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deicon/httprunner/parser"
	"github.com/deicon/httprunner/reporting"
	"github.com/deicon/httprunner/runner"
)

func main() {
	// Command line flags
	concurrency := flag.Int("u", 1, "Number of parallel virtual parallel users")
	iterations := flag.Int("i", 1, "Number of iterations")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")
	reportFormat := flag.String("report", "console", "Report format: console, html, csv, json")
	reportOutput := flag.String("output", "results", "Output directory for results and reports")
	reportDetail := flag.String("detail", "summary", "Report detail level: summary, goroutine, iteration, full")

	flag.Parse()

	if *requestFile == "" {
		fmt.Println("Error: -f flag is required")
		os.Exit(1)
	}

	// Validate report format
	format := reporting.ReportFormat(strings.ToLower(*reportFormat))
	switch format {
	case reporting.FormatConsole, reporting.FormatHTML, reporting.FormatCSV, reporting.FormatJSON:
		// valid format
	default:
		fmt.Printf("Error: Invalid report format '%s'. Valid formats: console, html, csv, json\n", *reportFormat)
		os.Exit(1)
	}

	// Validate report detail level
	detailLevel := reporting.ReportDetailLevel(strings.ToLower(*reportDetail))
	switch detailLevel {
	case reporting.DetailSummary, reporting.DetailVirtualuser, reporting.DetailIteration, reporting.DetailFull:
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
		r, err = runner.NewRunnerWithEnvFile(*concurrency, *iterations, *delay, requests, *envFile, *reportOutput)
	} else {
		r, err = runner.NewRunner(*concurrency, *iterations, *delay, requests, *reportOutput)
	}
	if err != nil {
		fmt.Printf("Error creating streaming runner: %v\n", err)
		os.Exit(1)
	}

	// Generate appropriate report based on detail level
	var reportContent string

	if detailLevel == reporting.DetailSummary {
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
		hierarchicalFormatter := &reporting.HierarchicalFormatter{
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
	if format == reporting.FormatConsole {
		// For console format, print to stdout
		fmt.Print(reportContent)
	} else {
		// For other formats, save to file
		filename := filepath.Join(*reportOutput, "report")
		switch format {
		case reporting.FormatHTML:
			filename += ".html"
		case reporting.FormatCSV:
			filename += ".csv"
		case reporting.FormatJSON:
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
