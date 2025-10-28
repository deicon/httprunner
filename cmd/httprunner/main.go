package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	offset := flag.Int("offset", 0, "Max random startup delay per VU in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")
	convert := flag.String("convert", "", "Convert input file to target format (e.g., k6) and print to stdout")
	reportFormat := flag.String("report", "console", "Report format: console, html, csv, json")
	reportOutput := flag.String("output", "results", "Output directory for results and reports")
	reportDetail := flag.String("detail", "summary", "Report detail level: summary, goroutine, iteration, full")
	verbose := flag.Bool("v", false, "Verbose mode: print request result JSON for each request")
	rawFile := flag.String("raw", "", "Path to raw results .jsonl file to generate report without executing")
	csvShortcut := flag.Bool("csv", false, "Shorthand to output CSV from -raw to stdout (forces -report=csv, -detail=summary)")
	experimentalNodeRuntime := flag.Bool("experimental-node-runtime", false, "Use experimental Node.js runtime for JavaScript execution")

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Offline reporting mode: when -raw is provided, skip execution
	offlineMode := *rawFile != ""
	if !offlineMode && *requestFile == "" {
		fmt.Println("Error: -f flag is required (or provide -raw to read a raw results file)")
		os.Exit(1)
	}

	// Conversion mode: when -convert is provided, parse and emit converted output
	if *convert != "" {
		requests, err := parser.Parse(*requestFile)
		if err != nil {
			fmt.Printf("Error parsing file: %v\n", err)
			os.Exit(1)
		}

		switch strings.ToLower(*convert) {
		case "k6":
			// Generate K6 script
			genOpts := struct {
				Iterations int
				DelayMS    int
				EnvFile    string
			}{
				Iterations: *iterations,
				DelayMS:    *delay,
				EnvFile:    *envFile,
			}
			// Import and call converter
			k6gen, err := func() (string, error) {
				// Local import to avoid unused import when not converting
				return requireK6Generate(requests, genOpts)
			}()
			if err != nil {
				fmt.Printf("Error generating k6 script: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(k6gen)
			return
		default:
			fmt.Printf("Error: unknown converter '%s'\n", *convert)
			os.Exit(1)
		}
	}

	// If -csv shorthand is provided, force CSV formatting and summary detail;
	// also prefer printing to stdout for easy redirection.
	forceStdout := false
	if *csvShortcut {
		*reportFormat = string(types.FormatCSV)
		*reportDetail = string(types.DetailSummary)
		forceStdout = true
		if !offlineMode {
			fmt.Println("Error: -csv requires -raw to be provided")
			os.Exit(1)
		}
	}

	// Validate runtime vs iterations parameters (only for execution mode)
	if !offlineMode {
		if *runtime < 0 {
			fmt.Println("Error: runtime (-r) must be 0 or positive")
			os.Exit(1)
		}
		if *iterations < 1 {
			fmt.Println("Error: iterations (-i) must be positive")
			os.Exit(1)
		}
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

	// If offline mode, build report from raw file and exit
	if offlineMode {
		// Build a FileReporter on the provided raw file
		fr := reporting.NewFileReporter(*rawFile)

		var reportContent string
		var err error

		if detailLevel == types.DetailSummary {
			rep, genErr := fr.GenerateReport(time.Time{}) // let reporter derive start/end times
			if genErr != nil {
				fmt.Printf("Error generating report from raw file: %v\n", genErr)
				os.Exit(1)
			}
			formatter := reporting.GetFormatter(types.ReportFormat(strings.ToLower(*reportFormat)))
			reportContent, err = formatter.Format(rep)
		} else {
			hr, genErr := fr.GenerateHierarchicalReport(time.Time{})
			if genErr != nil {
				fmt.Printf("Error generating hierarchical report from raw file: %v\n", genErr)
				os.Exit(1)
			}
			hierarchicalFormatter := &hierarchical.HierarchicalFormatter{
				DetailLevel: detailLevel,
				Format:      types.ReportFormat(strings.ToLower(*reportFormat)),
			}
			reportContent, err = hierarchicalFormatter.FormatHierarchical(hr)
		}

		if err != nil {
			fmt.Printf("Error formatting report: %v\n", err)
			os.Exit(1)
		}

		// Output handling mirrors execution mode
		format := types.ReportFormat(strings.ToLower(*reportFormat))
		if format == types.FormatConsole || forceStdout {
			fmt.Print(reportContent)
		} else {
			// Ensure output directory exists
			if err := os.MkdirAll(*reportOutput, 0755); err != nil {
				fmt.Printf("Error ensuring output directory: %v\n", err)
				os.Exit(1)
			}
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

		// In offline mode, also echo the raw file path for convenience
		fmt.Printf("Raw results used: %s\n", *rawFile)
		return
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

	// Set verbose flag
	r.SetVerbose(*verbose)
	// Toggle experimental Node runtime if requested
	r.EnableNodeRuntime(*experimentalNodeRuntime)
	if *experimentalNodeRuntime {
		modulePaths := make([]string, 0)
		seen := make(map[string]struct{})
		addPath := func(candidate string, requireDirCheck bool) {
			if candidate == "" {
				return
			}
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				absPath = filepath.Clean(candidate)
			} else {
				absPath = filepath.Clean(absPath)
			}
			if _, ok := seen[absPath]; ok {
				return
			}
			if requireDirCheck {
				info, statErr := os.Stat(absPath)
				if statErr != nil || !info.IsDir() {
					return
				}
			}
			seen[absPath] = struct{}{}
			modulePaths = append(modulePaths, absPath)
		}

		if !offlineMode && *requestFile != "" {
			dir := filepath.Dir(*requestFile)
			for depth := 0; depth < 3 && dir != ""; depth++ {
				nodeModulesPath := filepath.Join(dir, "node_modules")
				addPath(nodeModulesPath, true)
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}
		}

		if envPaths := os.Getenv("NODE_PATH"); envPaths != "" {
			for _, segment := range strings.Split(envPaths, string(os.PathListSeparator)) {
				trimmed := strings.TrimSpace(segment)
				if trimmed == "" {
					continue
				}
				addPath(trimmed, true)
			}
		}

		r.SetNodeRequirePaths(modulePaths)
	}
	// Apply startup offset between VU spawns, if provided
	r.StartupOffset = *offset

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
