package ai

import (
	"errors"
	"strings"
	"testing"
)

func TestAnalysisTemplatesValidateAndDoNotChangeOrdinaryChat(t *testing.T) {
	if text, err := templateInstruction(""); err != nil || text != "" {
		t.Fatal(text, err)
	}
	for id := range analysisTemplates {
		text, err := templateInstruction(id)
		if err != nil || !strings.Contains(text, "[analysis_template]") {
			t.Fatal(id, text, err)
		}
	}
	if _, err := templateInstruction("custom-shell"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("unknown template accepted", err)
	}
}
