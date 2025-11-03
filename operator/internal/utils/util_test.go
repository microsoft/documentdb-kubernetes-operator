// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

import (
	"testing"
)

func TestGenerateServiceName(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		target        string
		resourceGroup string
		expected      string
		description   string
	}{
		{
			name:          "short names within limit",
			source:        "us-east",
			target:        "us-west",
			resourceGroup: "rg1",
			expected:      "us-east-us-west",
			description:   "Names that fit within the 63-character limit should be returned as-is",
		},
		{
			name:          "empty resource group",
			source:        "eastus",
			target:        "westus",
			resourceGroup: "",
			expected:      "eastus-westus",
			description:   "Empty resource group should not affect the result",
		},
		{
			name:          "long resource group name",
			source:        "eastus",
			target:        "westus",
			resourceGroup: "very-long-resource-group-name-that-exceeds-normal-limits",
			expected:      "ea-we",
			description:   "Long resource group names will cause truncation when service name is short",
		},
		{
			name:          "names near character limit",
			source:        "abcdefghijklmnopqrstuvwxyz123456", // 32 chars
			target:        "abcdefghijklmnopqrstuvwxyz123456", // 32 chars, total with hyphen = 65
			resourceGroup: "",
			expected:      "abcdefghijklmnopqrstuvwxyz1234-abcdefghijklmnopqrstuvwxyz1234", // Should be truncated
			description:   "Names at the boundary should be truncated to fit",
		},
		{
			name:          "single character names",
			source:        "a",
			target:        "b",
			resourceGroup: "c",
			expected:      "a-b",
			description:   "Single character names should work correctly",
		},
		{
			name:          "moderate length names within limit",
			source:        "westeurope",
			target:        "eastus2",
			resourceGroup: "my-resource-group",
			expected:      "westeurope-eastus2",
			description:   "Moderate length names should not require truncation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateServiceName(tt.source, tt.target, tt.resourceGroup)
			if result != tt.expected {
				t.Errorf("generateServiceName(%q, %q, %q) = %q; expected %q\nDescription: %s",
					tt.source, tt.target, tt.resourceGroup, result, tt.expected, tt.description)
			}

			// Verify the result doesn't exceed reasonable length limits
			if len(result) > 63 {
				t.Errorf("GenerateServiceName(%q, %q, %q) returned a name longer than 63 characters: %q (length: %d)",
					tt.source, tt.target, tt.resourceGroup, result, len(result))
			}
		})
	}
}
