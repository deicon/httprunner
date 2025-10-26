package template

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

//go:embed node_worker.js
var nodeWorkerSource string

const (
	nodeResponseTypeResult    = "result"
	nodeResponseTypeAssertion = "assertion"
	nodeResponseTypeError     = "error"
	nodeMessageInvokeRequest  = "invoke_request"
	nodeMessageRequestResult  = "request_result"
	nodeMessageShutdownAck    = "shutdown_ack"
)

type nodeRuntime struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	tempDir string
	mu      sync.Mutex
	closed  bool
}

type nodeExecuteRequest struct {
	Type             string                 `json:"type"`
	Script           string                 `json:"script"`
	ResponseBody     interface{}            `json:"responseBody"`
	Context          map[string]interface{} `json:"context"`
	Globals          map[string]interface{} `json:"globals"`
	RequestFunctions []string               `json:"requestFunctions,omitempty"`
	RequirePaths     []string               `json:"requirePaths,omitempty"`
}

type nodeExecuteResponse struct {
	Type      string                 `json:"type"`
	Globals   map[string]interface{} `json:"globals"`
	Checks    []nodeCheck            `json:"checks"`
	Logs      []nodeLogEntry         `json:"logs"`
	Error     *nodeError             `json:"error,omitempty"`
	Assertion *nodeAssertion         `json:"assertion,omitempty"`
}

type nodeInvokeRequest struct {
	Type string        `json:"type"`
	ID   string        `json:"id"`
	Name string        `json:"name"`
	Args []interface{} `json:"args"`
}

type nodeRequestResultMessage struct {
	Type     string               `json:"type"`
	ID       string               `json:"id"`
	Success  bool                 `json:"success"`
	Response *nodeRequestResponse `json:"response,omitempty"`
	Error    string               `json:"error,omitempty"`
}

type nodeRequestResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type nodeRequestHandler func(name string, args []interface{}) (*nodeRequestResponse, error)

type nodeCheck struct {
	Name           string `json:"name"`
	Success        bool   `json:"success"`
	FailureMessage string `json:"failureMessage"`
}

type nodeLogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type nodeError struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

type nodeAssertion struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

func newNodeRuntime() (*nodeRuntime, error) {
	tempDir, err := os.MkdirTemp("", "httprunner-node-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir for node runtime: %w", err)
	}

	scriptPath := filepath.Join(tempDir, "worker.js")
	if writeErr := os.WriteFile(scriptPath, []byte(nodeWorkerSource), 0600); writeErr != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write node worker script: %w", writeErr)
	}

	cmd := exec.Command("node", scriptPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to get stdin pipe for node runtime: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to get stdout pipe for node runtime: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to start node runtime: %w", err)
	}

	return &nodeRuntime{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		tempDir: tempDir,
	}, nil
}

func (nr *nodeRuntime) Execute(req nodeExecuteRequest, handler nodeRequestHandler) (*nodeExecuteResponse, error) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	if nr.closed {
		return nil, errors.New("node runtime already closed")
	}

	if nr.cmd.ProcessState != nil && nr.cmd.ProcessState.Exited() {
		return nil, errors.New("node runtime process has exited")
	}

	if err := nr.sendMessage(req); err != nil {
		return nil, err
	}

	for {
		line, readErr := nr.stdout.ReadBytes('\n')
		if readErr != nil {
			return nil, fmt.Errorf("failed to read from node runtime: %w", readErr)
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("failed to decode node runtime response: %w", err)
		}

		switch envelope.Type {
		case nodeMessageInvokeRequest:
			var invocation nodeInvokeRequest
			if err := json.Unmarshal(line, &invocation); err != nil {
				return nil, fmt.Errorf("failed to decode invoke_request message: %w", err)
			}

			resultMessage := nodeRequestResultMessage{
				Type: nodeMessageRequestResult,
				ID:   invocation.ID,
			}

			if handler == nil {
				resultMessage.Success = false
				resultMessage.Error = "request handler not configured"
				resultMessage.Response = &nodeRequestResponse{
					Success: false,
					Error:   resultMessage.Error,
				}
			} else {
				response, err := handler(invocation.Name, invocation.Args)
				if err != nil {
					resultMessage.Success = false
					resultMessage.Error = err.Error()
					resultMessage.Response = &nodeRequestResponse{
						Success: false,
						Error:   resultMessage.Error,
					}
				} else if response == nil {
					resultMessage.Success = false
					resultMessage.Error = "request handler returned no response"
					resultMessage.Response = &nodeRequestResponse{
						Success: false,
						Error:   resultMessage.Error,
					}
				} else {
					resultMessage.Success = response.Success
					resultMessage.Response = response
					if response.Error != "" {
						resultMessage.Error = response.Error
					}
				}
			}

			if err := nr.sendMessage(resultMessage); err != nil {
				return nil, fmt.Errorf("failed to send request_result to node runtime: %w", err)
			}
		case nodeMessageRequestResult:
			// Request results should only be sent from Go to the Node worker.
			// Receiving them back indicates a protocol mismatch.
			return nil, errors.New("unexpected request_result received from node runtime")
		case nodeResponseTypeResult, nodeResponseTypeAssertion, nodeResponseTypeError:
			var resp nodeExecuteResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				return nil, fmt.Errorf("failed to decode node runtime response: %w", err)
			}
			return &resp, nil
		case nodeMessageShutdownAck:
			// Ignore shutdown acknowledgements while executing.
			continue
		default:
			return nil, fmt.Errorf("unexpected node runtime message type: %s", envelope.Type)
		}
	}
}

func (nr *nodeRuntime) Close() error {
	nr.mu.Lock()
	if nr.closed {
		nr.mu.Unlock()
		return nil
	}
	nr.closed = true
	nr.mu.Unlock()

	defer func() {
		_ = os.RemoveAll(nr.tempDir)
	}()

	shutdownPayload, _ := json.Marshal(map[string]string{"type": "shutdown"})
	shutdownPayload = append(shutdownPayload, '\n')

	_, _ = nr.stdin.Write(shutdownPayload)
	_ = nr.stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- nr.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		if nr.cmd.Process != nil {
			_ = nr.cmd.Process.Kill()
		}
		return errors.New("node runtime did not shut down gracefully")
	}
}

func (nr *nodeRuntime) sendMessage(message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal node runtime message: %w", err)
	}
	payload = append(payload, '\n')

	if _, err := nr.stdin.Write(payload); err != nil {
		return fmt.Errorf("failed to write to node runtime: %w", err)
	}
	return nil
}
