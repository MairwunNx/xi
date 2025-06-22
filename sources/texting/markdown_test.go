package texting

import (
	"testing"
)

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic escaping",
			input:    "Hello_world",
			expected: "Hello_world",
		},
		{
			name:     "Multiple special characters",
			input:    "Test[]()>#+-={}.!",
			expected: "Test\\[\\]\\(\\)>\\#\\+\\-\\=\\{\\}\\.\\!",
		},
		{
			name:     "Remove escaped headers",
			input:    "Text\n\n\\# Header",
			expected: "Text\n\nHeader",
		},
		{
			name:     "Remove multiple escaped headers",
			input:    "Text\n\n\\#\\#\\# Big Header",
			expected: "Text\n\nBig Header",
		},
		{
			name:     "Remove escaped quotes",
			input:    "Text\n\\> Quote",
			expected: "Text\n> Quote",
		},
		{
			name:     "Remove escaped collapsible quotes",
			input:    "Text\n**\\> Collapsible",
			expected: "Text\n**> Collapsible",
		},
		{
			name:     "Unescape links",
			input:    "Check \\[this link\\]\\(https://example.com\\)",
			expected: "Check [this link](https://example.com)",
		},
		{
			name:     "Complex test with all features",
			input:    "Hello_world!\n\n\\#\\# Title\n\\> Quote\n**\\> Collapsible\nLink: \\[text\\]\\(url\\)",
			expected: "Hello_world\\!\n\nTitle\n> Quote\n**> Collapsible\nLink: [text](url)",
		},
		{
			name:     "Headers with space",
			input:    "Text\n\n\\#\\#\\# Header with space",
			expected: "Text\n\nHeader with space",
		},
		{
			name:     "Headers without space",
			input:    "Text\n\n\\#\\#\\#HeaderNoSpace",
			expected: "Text\n\nHeaderNoSpace",
		},
		{
			name:     "Multiple lines with headers",
			input:    "First\n\n\\# Header1\nText\n\\#\\# Header2",
			expected: "First\n\nHeader1\nText\nHeader2",
		},
		{
			name:     "Mixed content",
			input:    "Text_with_underscores\n\n\\#\\# Header\n\\> Quote\nNormal text\n**\\> Collapsible\n\\[Link\\]\\(url\\)",
			expected: "Text_with_underscores\n\nHeader\n> Quote\nNormal text\n**> Collapsible\n[Link](url)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeMarkdown() = %q, expected %q", result, tt.expected)
			} else {
				t.Logf("PASSED")
			}
		})
	}
}