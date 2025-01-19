package maven

import (
	"testing"
)

func TestResolveMavenFields(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		properties map[string]string
		expected   string
	}{
		{
			name:       "NoPlaceholders",
			input:      "plain-text",
			properties: map[string]string{},
			expected:   "plain-text",
		},
		{
			name:  "SinglePlaceholder",
			input: "${version}",
			properties: map[string]string{
				"version": "1.0.0",
			},
			expected: "1.0.0",
		},
		{
			name:  "SinglePlaceholderWithDots",
			input: "${project.parent.version}",
			properties: map[string]string{
				"project.parent.version": "1.0.0",
			},
			expected: "1.0.0",
		},
		{
			name:  "MultiplePlaceholders",
			input: "${groupId}.${artifactId}:${version}",
			properties: map[string]string{
				"groupId":    "com.example",
				"artifactId": "example-project",
				"version":    "1.0.0",
			},
			expected: "com.example.example-project:1.0.0",
		},
		{
			name:  "UnknownPlaceholder",
			input: "${unknown}",
			properties: map[string]string{
				"version": "1.0.0",
			},
			expected: "${unknown}",
		},
		{
			name:  "MixedKnownAndUnknownPlaceholders",
			input: "${groupId}.${unknown}:${version}",
			properties: map[string]string{
				"groupId": "com.example",
				"version": "1.0.0",
			},
			expected: "com.example.${unknown}:1.0.0",
		},
		{
			name:       "EmptyPropertiesMap",
			input:      "${groupId}.${artifactId}:${version}",
			properties: map[string]string{},
			expected:   "${groupId}.${artifactId}:${version}",
		},
		{
			name:  "NestedPlaceholderIgnored",
			input: "${${nested}}",
			properties: map[string]string{
				"nested": "key",
				"key":    "value",
			},
			expected: "${key}",
		},
		{
			name:  "PlaceholderWithSpecialCharacters",
			input: "${weird-key_123}",
			properties: map[string]string{
				"weird-key_123": "special-value",
			},
			expected: "special-value",
		},
		{
			name:  "PlaceholderWithNoBraces",
			input: "normal-text",
			properties: map[string]string{
				"key": "value",
			},
			expected: "normal-text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveMavenFields(tt.input, tt.properties)
			if result != tt.expected {
				t.Errorf("expected: %s, got: %s", tt.expected, result)
			}
		})
	}
}
