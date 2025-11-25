// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

import (
	"testing"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestGenerateConnectionString(t *testing.T) {
	tests := []struct {
		name           string
		documentdb     *dbpreview.DocumentDB
		serviceIp      string
		trustTLS       bool
		expectedPrefix string
		expectedSuffix string
		description    string
	}{
		{
			name: "default secret with untrusted TLS",
			documentdb: &dbpreview.DocumentDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-db",
					Namespace: "test-namespace",
				},
				Spec: dbpreview.DocumentDBSpec{
					DocumentDbCredentialSecret: "",
				},
			},
			serviceIp:      "192.168.1.100",
			trustTLS:       false,
			expectedPrefix: "mongodb://$(kubectl get secret documentdb-credentials -n test-namespace -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret documentdb-credentials -n test-namespace -o jsonpath='{.data.password}' | base64 -d)@192.168.1.100:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true",
			expectedSuffix: "&tlsAllowInvalidCertificates=true&replicaSet=rs0",
			description:    "When no secret is specified, should use default secret and include tlsAllowInvalidCertificates",
		},
		{
			name: "custom secret with trusted TLS",
			documentdb: &dbpreview.DocumentDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-db",
					Namespace: "test-namespace",
				},
				Spec: dbpreview.DocumentDBSpec{
					DocumentDbCredentialSecret: "custom-secret",
				},
			},
			serviceIp:      "10.0.0.50",
			trustTLS:       true,
			expectedPrefix: "mongodb://$(kubectl get secret custom-secret -n test-namespace -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret custom-secret -n test-namespace -o jsonpath='{.data.password}' | base64 -d)@10.0.0.50:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true",
			expectedSuffix: "&replicaSet=rs0",
			description:    "When trustTLS is true, should not include tlsAllowInvalidCertificates",
		},
		{
			name: "hostname instead of IP with untrusted TLS",
			documentdb: &dbpreview.DocumentDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod-db",
					Namespace: "production",
				},
				Spec: dbpreview.DocumentDBSpec{
					DocumentDbCredentialSecret: "prod-credentials",
				},
			},
			serviceIp:      "documentdb.example.com",
			trustTLS:       false,
			expectedPrefix: "mongodb://$(kubectl get secret prod-credentials -n production -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret prod-credentials -n production -o jsonpath='{.data.password}' | base64 -d)@documentdb.example.com:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true",
			expectedSuffix: "&tlsAllowInvalidCertificates=true&replicaSet=rs0",
			description:    "Should work with hostname/FQDN instead of IP address",
		},
		{
			name: "IPv6 address with trusted TLS",
			documentdb: &dbpreview.DocumentDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ipv6-db",
					Namespace: "default",
				},
				Spec: dbpreview.DocumentDBSpec{
					DocumentDbCredentialSecret: "ipv6-secret",
				},
			},
			serviceIp:      "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			trustTLS:       true,
			expectedPrefix: "mongodb://$(kubectl get secret ipv6-secret -n default -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret ipv6-secret -n default -o jsonpath='{.data.password}' | base64 -d)@2001:0db8:85a3:0000:0000:8a2e:0370:7334:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true",
			expectedSuffix: "&replicaSet=rs0",
			description:    "Should support IPv6 addresses",
		},
		{
			name: "different namespace with custom secret",
			documentdb: &dbpreview.DocumentDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-db",
					Namespace: "app-namespace",
				},
				Spec: dbpreview.DocumentDBSpec{
					DocumentDbCredentialSecret: "app-secret",
				},
			},
			serviceIp:      "172.16.0.10",
			trustTLS:       false,
			expectedPrefix: "mongodb://$(kubectl get secret app-secret -n app-namespace -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret app-secret -n app-namespace -o jsonpath='{.data.password}' | base64 -d)@172.16.0.10:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true",
			expectedSuffix: "&tlsAllowInvalidCertificates=true&replicaSet=rs0",
			description:    "Should correctly use the DocumentDB instance's namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateConnectionString(tt.documentdb, tt.serviceIp, tt.trustTLS)

			// Verify the connection string starts with expected prefix
			expectedFull := tt.expectedPrefix + tt.expectedSuffix
			if result != expectedFull {
				t.Errorf("GenerateConnectionString() = %q; expected %q\nDescription: %s",
					result, expectedFull, tt.description)
			}

			// Verify essential components are present
			if result == "" {
				t.Error("GenerateConnectionString() returned empty string")
			}

			// Verify it contains mongodb://
			if len(result) < 10 || result[:10] != "mongodb://" {
				t.Errorf("Connection string should start with 'mongodb://', got: %q", result[:10])
			}

			// Verify TLS parameter is present
			if !contains(result, "tls=true") {
				t.Error("Connection string should contain 'tls=true'")
			}

			// Verify SCRAM-SHA-256 auth mechanism
			if !contains(result, "authMechanism=SCRAM-SHA-256") {
				t.Error("Connection string should contain 'authMechanism=SCRAM-SHA-256'")
			}

			// Verify replicaSet parameter
			if !contains(result, "replicaSet=rs0") {
				t.Error("Connection string should contain 'replicaSet=rs0'")
			}

			// Verify tlsAllowInvalidCertificates based on trustTLS
			if tt.trustTLS {
				if contains(result, "tlsAllowInvalidCertificates") {
					t.Error("Connection string should NOT contain 'tlsAllowInvalidCertificates' when trustTLS is true")
				}
			} else {
				if !contains(result, "tlsAllowInvalidCertificates=true") {
					t.Error("Connection string should contain 'tlsAllowInvalidCertificates=true' when trustTLS is false")
				}
			}

			// Verify service IP is in the connection string
			if !contains(result, tt.serviceIp) {
				t.Errorf("Connection string should contain service IP/hostname %q", tt.serviceIp)
			}

			// Verify namespace is used correctly
			if !contains(result, tt.documentdb.Namespace) {
				t.Errorf("Connection string should contain namespace %q", tt.documentdb.Namespace)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
