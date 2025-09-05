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
	concurrency := flag.Int("t", 1, "Number of parallel go routines")
	iterations := flag.Int("i", 1, "Number of iterations")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")
	reportFormat := flag.String("report", "html", "Report format: console, html, csv, json")
	reportOutput := flag.String("output", "", "Output directory for streaming results and reports (enables streaming mode)")
	reportDetail := flag.String("detail", "full", "Report detail level: summary, goroutine, iteration, full")
	streaming := flag.Bool("stream", false, "Enable streaming mode to reduce memory usage (requires -output)")

	flag.Parse()

	if *requestFile == "" {
		fmt.Println("Error: -f flag is required")
		os.Exit(1)
	}

	// Validate streaming mode requirements
	if *streaming && *reportOutput == "" {
		fmt.Println("Error: -output directory is required when using streaming mode (-stream)")
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

	// Determine if we should use streaming mode (based on -stream flag or large workload)
	useStreaming := *streaming || shouldUseStreaming(*concurrency, *iterations, len(requests))

	if useStreaming {
		// Use streaming runner for memory efficiency
		outputDir := *reportOutput
		if outputDir == "" {
			outputDir = "results" // default output directory for auto-streaming
		}

		if *envFile != "" {
			r, err = runner.NewStreamingRunnerWithEnvFile(*concurrency, *iterations, *delay, requests, *envFile, outputDir)
		} else {
			r, err = runner.NewStreamingRunner(*concurrency, *iterations, *delay, requests, outputDir)
		}
		if err != nil {
			fmt.Printf("Error creating streaming runner: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Using streaming mode - results will be written to: %s\n", outputDir)
	} else {
		// Use traditional in-memory runner
		if *envFile != "" {
			r, err = runner.NewRunnerWithEnvFile(*concurrency, *iterations, *delay, requests, *envFile)
			if err != nil {
				fmt.Printf("Error loading env file: %v\n", err)
				os.Exit(1)
			}
		} else {
			r = runner.NewRunner(*concurrency, *iterations, *delay, requests)
		}
	}

	// Generate appropriate report based on detail level and runner type
	var reportContent string

	if useStreaming {
		// Use streaming execution
		if detailLevel == reporting.DetailSummary {
			report, streamErr := r.RunStreaming()
			if streamErr != nil {
				fmt.Printf("Error running streaming execution: %v\n", streamErr)
				os.Exit(1)
			}
			formatter := reporting.GetFormatter(format)
			reportContent, err = formatter.Format(report)
		} else {
			hierarchicalReport, streamErr := r.RunHierarchicalStreaming()
			if streamErr != nil {
				fmt.Printf("Error running hierarchical streaming execution: %v\n", streamErr)
				os.Exit(1)
			}
			hierarchicalFormatter := &reporting.HierarchicalFormatter{
				DetailLevel: detailLevel,
				Format:      format,
			}
			reportContent, err = hierarchicalFormatter.FormatHierarchical(hierarchicalReport)
		}
	} else {
		// Use traditional in-memory execution
		if detailLevel == reporting.DetailSummary {
			report := r.Run()
			formatter := reporting.GetFormatter(format)
			reportContent, err = formatter.Format(report)
		} else {
			hierarchicalReport := r.RunHierarchical()
			hierarchicalFormatter := &reporting.HierarchicalFormatter{
				DetailLevel: detailLevel,
				Format:      format,
			}
			reportContent, err = hierarchicalFormatter.FormatHierarchical(hierarchicalReport)
		}
	}

	if err != nil {
		fmt.Printf("Error formatting report: %v\n", err)
		os.Exit(1)
	}

	// Output report
	if *reportOutput != "" && !useStreaming {
		// Generate filename based on format if no extension provided (traditional mode)
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
	} else if useStreaming && *reportOutput != "" {
		// In streaming mode, save formatted report to output directory
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
		fmt.Printf("Raw results available in: %s/raw-results-*.jsonl\n", *reportOutput)
	} else {
		// Print to stdout
		fmt.Print(reportContent)
	}
}

// shouldUseStreaming determines if streaming should be used based on workload size
func shouldUseStreaming(concurrency, iterations, requestCount int) bool {
	// Use streaming for large workloads that could consume significant memory
	totalOperations := concurrency * iterations * requestCount
	return totalOperations > 10000 // Threshold for auto-streaming
}
