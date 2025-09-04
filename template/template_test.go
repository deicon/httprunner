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
	_ = os.Setenv("TEST_ENV_VAR", "env_value")
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

func TestJavaScriptStringMethods(t *testing.T) {
	engine := NewTemplateEngine()

	// Test JavaScript string methods on response body
	jsonResponse := `{"components":[],"processId":"f9b70d8c-5874-4f1f-9045-02a63034d95e","hauptkunde":"111787383","gesetzlicherVertreterEins":"99695265","gesetzlicherVertreterZwei":"99695264","minderjaehrig":true,"product":"Junges_Konto","art":"EINZEL","subProcesses":[{"type":"investmentdialog","parameter":[{"name":"editPath","value":"/frontend/v3/767fa6d5-ed4f-4e6f-9f6f-a0dcdbcd212b"}]}],"haushaltsId":"50990592","status":"VORSCHLAG_BERECHNET","hasDepot":true,"useEem":false}`
	script := `
		var data = response.body;
		
		// Test substring method
		var editPath = data.subProcesses[0].parameter[0]['value']
		
		// Test lastIndexOf method
		var lastIndex = editPath.lastIndexOf("/");
		client.global.set("lastIndexOf_result", lastIndex);
		
		// Test combined usage - extract file extension-like pattern
		var pathParts = editPath.substring(editPath.lastIndexOf("/") + 1);
		client.global.set("path_end", pathParts);
	`

	err := engine.ExecuteScript(script, jsonResponse)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Verify lastIndexOf result
	lastIndexResult := engine.GetGlobalStore().Get("lastIndexOf_result")
	if lastIndexResult != int64(12) { // JavaScript numbers are int64 in Go through Goja
		t.Errorf("Expected 12, got %v (type: %T)", lastIndexResult, lastIndexResult)
	}

	// Verify combined usage result
	pathEnd := engine.GetGlobalStore().Get("path_end")
	if pathEnd != "767fa6d5-ed4f-4e6f-9f6f-a0dcdbcd212b" {
		t.Errorf("Expected '767fa6d5-ed4f-4e6f-9f6f-a0dcdbcd212b', got %v", pathEnd)
	}
}
