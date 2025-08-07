package template

import (
	"os"
	"testing"
)

func TestGlobalStore(t *testing.T) {
	store := NewGlobalStore()

	// Test setting and getting values
	store.Set("test_key", "test_value")
	value := store.Get("test_key")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test environment variables are loaded
	os.Setenv("TEST_ENV_VAR", "env_value")
	store = NewGlobalStore()
	envValue := store.Get("TEST_ENV_VAR")
	if envValue != "env_value" {
		t.Errorf("Expected 'env_value', got %v", envValue)
	}
}

func TestTemplateEngine(t *testing.T) {
	engine := NewTemplateEngine()
	engine.GetGlobalStore().Set("TEST_VAR", "test_value")

	// Test template rendering
	result, err := engine.RenderTemplate("Hello {{.TEST_VAR}}")
	if err != nil {
		t.Errorf("Template rendering failed: %v", err)
	}
	if result != "Hello test_value" {
		t.Errorf("Expected 'Hello test_value', got '%s'", result)
	}
}

func TestJavaScriptExecution(t *testing.T) {
	engine := NewTemplateEngine()

	// Test JavaScript execution with JSON response
	jsonResponse := `{"id": 123, "name": "test"}`
	script := `
		var data = response.body;
		client.global.set("extracted_id", data.id);
		client.global.set("extracted_name", data.name);
	`

	err := engine.ExecuteScript(script, jsonResponse)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Verify values were set
	id := engine.GetGlobalStore().Get("extracted_id")
	if id != int64(123) { // JavaScript integers are int64 in Go through Goja
		t.Errorf("Expected 123, got %v (type: %T)", id, id)
	}

	name := engine.GetGlobalStore().Get("extracted_name")
	if name != "test" {
		t.Errorf("Expected 'test', got %v", name)
	}
}
