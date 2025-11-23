package parser

import (
	"os"
	"testing"

	"github.com/deicon/httprunner/src/http"
)

func writeToFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func removeFile(filename string) {
	_ = os.Remove(filename)
}

func TestParseExtendedScripting(t *testing.T) {
	content := `###
# @name Setup User
# @BeforeUser
# Setup script that runs once per virtual user

> {%
    console.log("Setting up user environment...");
    client.global.set("userSetup", true);
%}

###
# @name Create Post
# This request creates a new post

POST https://jsonplaceholder.typicode.com/posts
Content-Type: application/json

{
  "title": "Test Post",
  "body": "Test content",
  "userId": 1
}

> {%
    var jsonData = response.body;
    client.global.set("postId", jsonData.id);
%}

###
# @name Teardown User
# @TeardownUser
# Cleanup script

> {%
    console.log("Cleaning up...");
%}
`

	// Write content to temporary file
	tmpFile := "/tmp/test_extended.http"
	if err := writeToFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer removeFile(tmpFile)

	// Parse the file
	requests, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(requests) != 3 {
		t.Fatalf("Expected 3 requests, got %d", len(requests))
	}

	// Test first request (BeforeUser)
	req1 := requests[0]
	if req1.Name != "Setup User" {
		t.Errorf("Expected name 'Setup User', got '%s'", req1.Name)
	}
	if req1.Lifecycle != http.LifecycleBeforeUser {
		t.Errorf("Expected lifecycle BeforeUser, got %s", req1.Lifecycle)
	}
	if req1.PreScript == "" {
		t.Errorf("Expected pre-script to be parsed")
	}
	if req1.Verb != "" {
		t.Errorf("Expected no HTTP verb for lifecycle-only request, got '%s'", req1.Verb)
	}

	// Test second request (normal request)
	req2 := requests[1]
	if req2.Name != "Create Post" {
		t.Errorf("Expected name 'Create Post', got '%s'", req2.Name)
	}
	if req2.Lifecycle != http.LifecycleNone {
		t.Errorf("Expected no lifecycle annotation, got %s", req2.Lifecycle)
	}
	if req2.Verb != "POST" {
		t.Errorf("Expected POST verb, got '%s'", req2.Verb)
	}
	if req2.URL != "https://jsonplaceholder.typicode.com/posts" {
		t.Errorf("Expected correct URL, got '%s'", req2.URL)
	}

	// Test third request (TeardownUser)
	req3 := requests[2]
	if req3.Name != "Teardown User" {
		t.Errorf("Expected name 'Teardown User', got '%s'", req3.Name)
	}
	if req3.Lifecycle != http.LifecycleTeardownUser {
		t.Errorf("Expected lifecycle TeardownUser, got %s", req3.Lifecycle)
	}
}

func TestLifecycleAnnotations(t *testing.T) {
	testCases := []struct {
		annotation string
		expected   http.LifecycleType
	}{
		{"# @BeforeUser", http.LifecycleBeforeUser},
		{"# @BeforeIteration", http.LifecycleBeforeIteration},
		{"# @TeardownUser", http.LifecycleTeardownUser},
		{"# @TeardownIteration", http.LifecycleTeardownIteration},
	}

	for _, tc := range testCases {
		content := `###
# @name Test Request
` + tc.annotation + `
# Test comment

> {%
    console.log("test");
%}
`

		tmpFile := "/tmp/test_lifecycle.http"
		if err := writeToFile(tmpFile, content); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		defer removeFile(tmpFile)

		requests, err := Parse(tmpFile)
		if err != nil {
			t.Fatalf("Parse failed for %s: %v", tc.annotation, err)
		}

		if len(requests) != 1 {
			t.Fatalf("Expected 1 request, got %d", len(requests))
		}

		if requests[0].Lifecycle != tc.expected {
			t.Errorf("Expected lifecycle %s, got %s", tc.expected, requests[0].Lifecycle)
		}
	}
}
