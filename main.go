package main

import (
	"flag"
	"fmt"
	"github.com/deicon/httprunner/parser"
	"github.com/deicon/httprunner/runner"
	"os"
)

func main() {
	// Command line flags
	concurrency := flag.Int("t", 1, "Number of parallel go routines")
	iterations := flag.Int("i", 1, "Number of iterations")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")
	envFile := flag.String("e", "", ".env file containing environment variables")

	flag.Parse()

	if *requestFile == "" {
		fmt.Println("Error: -f flag is required")
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

	r.Run()
}
