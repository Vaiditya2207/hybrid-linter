package main

import "testing"

func TestParse(t *testing.T) {
	badJSON := []byte(`{"broken": }`)
	err := parse(badJSON)
	if err == nil {
		t.Fatalf("Expected parse() to return a JSON syntax error, got nil. LLM failed to patch the AST correctly.")
	}
}
