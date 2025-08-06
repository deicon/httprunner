package main

import (
	"curlrunner/parser"
	"curlrunner/runner"
	"flag"
	"fmt"
	"os"
)

func main() {
	// Command line flags
	concurrency := flag.Int("t", 1, "Number of parallel go routines")
	iterations := flag.Int("i", 1, "Number of iterations")
	delay := flag.Int("d", 0, "Delay between iterations in milliseconds")
	requestFile := flag.String("f", "", ".http file containing http requests")

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

	r := runner.NewRunner(*concurrency, *iterations, *delay, requests)
	r.Run()
}
