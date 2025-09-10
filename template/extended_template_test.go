package template

import (
	"testing"

	"github.com/deicon/httprunner/http"
)

func TestConvertNameToFunctionName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Create Post", "create_post"},
		{"Get User Details", "get_user_details"},
		{"Delete-Item", "delete_item"},
		{"Update User Info!", "update_user_info"},
		{"Test   Multiple    Spaces", "test_multiple_spaces"},
		{"123 Numbers", "123_numbers"},
		{"Special!@#$%Characters", "special_characters"},
	}

	for _, tc := range testCases {
		result := convertNameToFunctionName(tc.input)
		if result != tc.expected {
			t.Errorf("convertNameToFunctionName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestRegisterRequestFunction(t *testing.T) {
	engine := NewTemplateEngine()

	// Test request with name
	namedRequest := http.Request{
		Name: "Test Request",
		Verb: "GET",
		URL:  "http://example.com",
	}

	engine.RegisterRequestFunction(namedRequest)

	// Check if function was registered
	if len(engine.requestFunctions) != 1 {
		t.Errorf("Expected 1 function registered, got %d", len(engine.requestFunctions))
	}

	functionName := convertNameToFunctionName("Test Request")
	if _, exists := engine.requestFunctions[functionName]; !exists {
		t.Errorf("Expected function %s to be registered", functionName)
	}

	// Test request without name (should not be registered)
	unnamedRequest := http.Request{
		Verb: "POST",
		URL:  "http://example.com",
	}

	engine.RegisterRequestFunction(unnamedRequest)

	// Should still be 1 function
	if len(engine.requestFunctions) != 1 {
		t.Errorf("Expected 1 function registered, got %d", len(engine.requestFunctions))
	}
}

func TestExecuteScriptWithFunctions(t *testing.T) {
	engine := NewTemplateEngine()

	// Register a test request
	testRequest := http.Request{
		Name: "Test API",
		Verb: "GET",
		URL:  "http://example.com/test",
	}
	engine.RegisterRequestFunction(testRequest)

	// Set up a mock request executor
	executorCalled := false
	engine.SetRequestExecutor(func(request http.Request) (*Response, error) {
		executorCalled = true
		if request.Name != "Test API" {
			t.Errorf("Expected request name 'Test API', got '%s'", request.Name)
		}
		return &Response{
			StatusCode: 200,
			Body:       map[string]interface{}{"success": true},
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	})

	// Test script that calls the generated function
	script := `
		var response = client.test_api();
		client.global.set("functionCallResult", response.success);
		client.global.set("statusCode", response.status_code);
	`

	err := engine.ExecuteScript(script, "", 0, 0)
	if err != nil {
		t.Fatalf("ExecuteScript failed: %v", err)
	}

	// Check if executor was called
	if !executorCalled {
		t.Error("Expected request executor to be called")
	}

	// Check if values were set correctly
	if engine.globalStore.Get("functionCallResult") != true {
		t.Error("Expected functionCallResult to be true")
	}

	// JavaScript numbers might be converted to int64 or float64, so check both
	statusCode := engine.globalStore.Get("statusCode")
	if statusCode != 200 && statusCode != int64(200) && statusCode != float64(200) {
		t.Errorf("Expected statusCode to be 200, got %v (type: %T)", statusCode, statusCode)
	}
}
