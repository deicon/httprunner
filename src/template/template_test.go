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

	err := engine.ExecuteScript(script, jsonResponse, 0, 0)
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

	err := engine.ExecuteScript(script, jsonResponse, 0, 0)
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

func TestCheckFunction(t *testing.T) {
	engine := NewTemplateEngine()

	// Test successful check
	script := `
		client.check("Status Code Check", function() {
			return response.body.status === "VORSCHLAG_BERECHNET";
		}, "Status should be VORSCHLAG_BERECHNET");
	`
	jsonResponse := `{"status":"VORSCHLAG_BERECHNET","id":123}`

	err := engine.ExecuteScript(script, jsonResponse, 0, 0)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Get check results
	checks := engine.GetChecks()
	if len(checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checks))
	}

	check := checks[0]
	if check.Name != "Status Code Check" {
		t.Errorf("Expected check name 'Status Code Check', got '%s'", check.Name)
	}
	if !check.Success {
		t.Errorf("Expected check to succeed, but it failed")
	}
	if check.FailureMessage != "Status should be VORSCHLAG_BERECHNET" {
		t.Errorf("Expected failure message 'Status should be VORSCHLAG_BERECHNET', got '%s'", check.FailureMessage)
	}
}

func TestCheckFunctionFailure(t *testing.T) {
	engine := NewTemplateEngine()

	// Test failed check
	script := `
		client.check("ID Validation", function() {
			return response.body.id > 1000;
		}, "ID should be greater than 1000");
	`
	jsonResponse := `{"id":123,"status":"OK"}`

	err := engine.ExecuteScript(script, jsonResponse, 0, 0)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Get check results
	checks := engine.GetChecks()
	if len(checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checks))
	}

	check := checks[0]
	if check.Name != "ID Validation" {
		t.Errorf("Expected check name 'ID Validation', got '%s'", check.Name)
	}
	if check.Success {
		t.Errorf("Expected check to fail, but it succeeded")
	}
	if check.FailureMessage != "ID should be greater than 1000" {
		t.Errorf("Expected failure message 'ID should be greater than 1000', got '%s'", check.FailureMessage)
	}
}

func TestMultipleChecks(t *testing.T) {
	engine := NewTemplateEngine()

	// Test multiple checks in one script
	script := `
		client.check("Status Check", function() {
			return response.body.status === "OK";
		}, "Status should be OK");
		
		client.check("ID Range Check", function() {
			return response.body.id >= 100 && response.body.id <= 200;
		}, "ID should be between 100 and 200");
		
		client.check("Name Present", function() {
			return response.body.name && response.body.name.length > 0;
		}, "Name should be present and non-empty");
	`
	jsonResponse := `{"id":150,"status":"OK","name":"test user"}`

	err := engine.ExecuteScript(script, jsonResponse, 0, 0)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Get check results
	checks := engine.GetChecks()
	if len(checks) != 3 {
		t.Errorf("Expected 3 checks, got %d", len(checks))
	}

	// Verify all checks passed
	for i, check := range checks {
		if !check.Success {
			t.Errorf("Check %d ('%s') should have succeeded but failed: %s", i, check.Name, check.FailureMessage)
		}
	}

	// Check specific check names
	expectedNames := []string{"Status Check", "ID Range Check", "Name Present"}
	for i, expectedName := range expectedNames {
		if checks[i].Name != expectedName {
			t.Errorf("Expected check %d name '%s', got '%s'", i, expectedName, checks[i].Name)
		}
	}
}

func TestCheckFunctionClearance(t *testing.T) {
	engine := NewTemplateEngine()

	// Add some checks
	script := `
		client.check("Test Check", function() { return true; }, "Test message");
	`

	err := engine.ExecuteScript(script, `{}`, 0, 0)
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Verify check was added
	checks := engine.GetChecks()
	if len(checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checks))
	}

	// Verify GetChecks() clears the internal list
	checksAgain := engine.GetChecks()
	if len(checksAgain) != 0 {
		t.Errorf("Expected 0 checks after second GetChecks() call, got %d", len(checksAgain))
	}
}
