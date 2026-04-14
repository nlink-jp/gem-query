package security

import (
	"strings"
	"testing"
)

func TestWrapPrompt(t *testing.T) {
	sys, wrapped, err := WrapPrompt("show me top 10 sales")
	if err != nil {
		t.Fatalf("WrapPrompt: %v", err)
	}

	if !strings.Contains(sys, "SQL query generator") {
		t.Error("system prompt should contain SQL query generator instruction")
	}
	if strings.Contains(sys, "{{DATA_TAG}}") {
		t.Error("system prompt should not contain unexpanded {{DATA_TAG}}")
	}
	if !strings.Contains(wrapped, "show me top 10 sales") {
		t.Error("wrapped prompt should contain original user input")
	}
	if !strings.Contains(wrapped, "<") || !strings.Contains(wrapped, ">") {
		t.Error("wrapped prompt should contain XML tags")
	}
}

func TestWrapPrompt_DifferentNonces(t *testing.T) {
	sys1, _, _ := WrapPrompt("query 1")
	sys2, _, _ := WrapPrompt("query 2")

	if sys1 == sys2 {
		t.Error("each call should produce a different nonce")
	}
}
