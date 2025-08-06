package parser

import (
	"curlrunner/http"
	"io/ioutil"
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
"mandant": "DVAG",
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
	tmpfile, err := ioutil.TempFile("", "test.http")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

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
"mandant": "DVAG",
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
