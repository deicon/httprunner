package reporting

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StreamingCollector writes results to files as they arrive to reduce memory usage
type StreamingCollector struct {
	outputDir   string
	resultFile  *os.File
	writer      *bufio.Writer
	startTime   time.Time
	resultCount int
}

// NewStreamingCollector creates a new streaming collector that writes to files
func NewStreamingCollector(outputDir string) (*StreamingCollector, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create raw results file with unique identifier to prevent collisions
	timestamp := time.Now().Format("20060102-150405")
	uniqueID, err := generateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique ID: %v", err)
	}
	resultFile, err := os.Create(filepath.Join(outputDir, fmt.Sprintf("raw-results-%s-%s.jsonl", timestamp, uniqueID)))
	if err != nil {
		return nil, fmt.Errorf("failed to create results file: %v", err)
	}

	return &StreamingCollector{
		outputDir:   outputDir,
		resultFile:  resultFile,
		writer:      bufio.NewWriter(resultFile),
		startTime:   time.Now(),
		resultCount: 0,
	}, nil
}

// AddResult writes a request result immediately to file
func (sc *StreamingCollector) AddResult(result RequestResult) error {
	// Convert result to JSON and write to file
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %v", err)
	}

	// Write JSON line to file
	if _, err := sc.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write result: %v", err)
	}
	if _, err := sc.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %v", err)
	}

	sc.resultCount++

	// Flush every 100 results to ensure data is written
	if sc.resultCount%100 == 0 {
		if err := sc.writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush writer: %v", err)
		}
	}

	return nil
}

// Close finalizes the streaming collector and closes files
func (sc *StreamingCollector) Close() error {
	if sc.writer != nil {
		if err := sc.writer.Flush(); err != nil {
			return err
		}
	}
	if sc.resultFile != nil {
		return sc.resultFile.Close()
	}
	return nil
}

// GetResultsFilePath returns the path to the raw results file
func (sc *StreamingCollector) GetResultsFilePath() string {
	return sc.resultFile.Name()
}

// GetResultCount returns the number of results written
func (sc *StreamingCollector) GetResultCount() int {
	return sc.resultCount
}

// GetStartTime returns the start time of collection
func (sc *StreamingCollector) GetStartTime() time.Time {
	return sc.startTime
}

// generateUniqueID creates a random 8-character hex string for file uniqueness
func generateUniqueID() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
