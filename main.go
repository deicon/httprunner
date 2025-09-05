package main

import (
	"flag"
	"fmt"
	"github.com/deicon/httprunner/parser"
	"github.com/deicon/httprunner/reporting"
	"github.com/deicon/httprunner/runner"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Command line flags
	concurrency := flag.Int("t", 1, "Number of parallel go routines")
	iterations := flag.Int("i", 1, "Number of iterations")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")
	reportFormat := flag.String("report", "console", "Report format: console, html, csv, json")
	reportOutput := flag.String("output", "", "Output file for report (optional, prints to stdout if not specified)")
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
	case reporting.DetailSummary, reporting.DetailGoroutine, reporting.DetailIteration, reporting.DetailFull:
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
	if *envFile != "" {
		r, err = runner.NewRunnerWithEnvFile(*concurrency, *iterations, *delay, requests, *envFile)
		if err != nil {
			fmt.Printf("Error loading env file: %v\n", err)
			os.Exit(1)
		}
	} else {
		r = runner.NewRunner(*concurrency, *iterations, *delay, requests)
	}

	// Generate appropriate report based on detail level
	var reportContent string

	if detailLevel == reporting.DetailSummary {
		// Use traditional report for summary level
		report := r.Run()
		formatter := reporting.GetFormatter(format)
		reportContent, err = formatter.Format(report)
	} else {
		// Use hierarchical report for detailed levels
		hierarchicalReport := r.RunHierarchical()
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

	// Output report
	if *reportOutput != "" {
		// Generate filename based on format if no extension provided
		filename := *reportOutput
		if filepath.Ext(filename) == "" {
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
		}

		if err := os.WriteFile(filename, []byte(reportContent), 0644); err != nil {
			fmt.Printf("Error writing report to file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nReport saved to: %s\n", filename)
	} else {
		// Print to stdout
		fmt.Print(reportContent)
	}
}
