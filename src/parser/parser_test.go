package parser

import (
	"github.com/deicon/httprunner/src/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	content := `
###
POST localhost:8080/api/v3/tarifrechner
Content-Type: application/json

{
"tarifrechnerModus": {
"modus": "TARIFRECHNER",
"mandant": "ORGA",
"haushaltsId": 48296349
},
"kundennummern": [87468640],
"produktKonfigurationId": "investmentanlage",
"vertragsId": 7007787476
}

###
GET http://example.com/test
Accept: application/json

###
`
	tmpfile, err := os.CreateTemp("", "test.http")
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	requests, err := Parse(tmpfile.Name())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	expected := []http.Request{
		{
			Verb: "POST",
			URL:  "localhost:8080/api/v3/tarifrechner",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{
"tarifrechnerModus": {
"modus": "TARIFRECHNER",
"mandant": "ORGA",
"haushaltsId": 48296349
},
"kundennummern": [87468640],
"produktKonfigurationId": "investmentanlage",
"vertragsId": 7007787476
}`,
		},
		{
			Verb: "GET",
			URL:  "http://example.com/test",
			Headers: map[string]string{
				"Accept": "application/json",
			},
			Body: "",
		},
	}

	if len(requests) != len(expected) {
		t.Fatalf("expected %d requests, got %d", len(expected), len(requests))
	}

	for i := range requests {
		// Trim space from body to avoid issues with line endings
		requests[i].Body = strings.TrimSpace(requests[i].Body)
		expected[i].Body = strings.TrimSpace(expected[i].Body)
		if !reflect.DeepEqual(requests[i], expected[i]) {
			t.Errorf("request %d: expected %+v, got %+v", i, expected[i], requests[i])
		}
	}
}

func TestParseWithPreRequestScript(t *testing.T) {
	content := `
###
# @name Test Request with Pre-script
> {%
client.global.set("dynamicValue", "test123")
%}

POST {{.BASEURL}}/api/test
Content-Type: application/json
Authorization: Bearer {{.TOKEN}}

{
  "value": "{{.dynamicValue}}"
}

> {%
var jsonData = response.body
client.global.set("responseId", jsonData.id)
%}

###
`
	tmpfile, err := os.CreateTemp("", "test-prescript.http")
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	requests, err := Parse(tmpfile.Name())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	expected := []http.Request{
		{
			Name: "Test Request with Pre-script",
			Verb: "POST",
			URL:  "{{.BASEURL}}/api/test",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{.TOKEN}}",
			},
			Body: `{
  "value": "{{.dynamicValue}}"
}`,
			PreScript: `client.global.set("dynamicValue", "test123")`,
			Script: `var jsonData = response.body
client.global.set("responseId", jsonData.id)`,
		},
	}

	if len(requests) != len(expected) {
		t.Fatalf("expected %d requests, got %d", len(expected), len(requests))
	}

	for i := range requests {
		// Trim space from body and scripts to avoid issues with line endings
		requests[i].Body = strings.TrimSpace(requests[i].Body)
		requests[i].PreScript = strings.TrimSpace(requests[i].PreScript)
		requests[i].Script = strings.TrimSpace(requests[i].Script)
		expected[i].Body = strings.TrimSpace(expected[i].Body)
		expected[i].PreScript = strings.TrimSpace(expected[i].PreScript)
		expected[i].Script = strings.TrimSpace(expected[i].Script)
		if !reflect.DeepEqual(requests[i], expected[i]) {
			t.Errorf("request %d: expected %+v, got %+v", i, expected[i], requests[i])
		}
	}
}
