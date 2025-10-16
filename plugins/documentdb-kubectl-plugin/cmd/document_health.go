package cmd

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func isDocumentReady(doc *unstructured.Unstructured, targetCluster string) bool {
	if doc == nil {
		return false
	}

	primary, _, err := unstructured.NestedString(doc.Object, "spec", "clusterReplication", "primary")
	if err != nil || primary != targetCluster {
		return false
	}

	healthy, _ := isDocumentHealthy(doc)
	return healthy
}

func isDocumentHealthy(doc *unstructured.Unstructured) (bool, string) {
	if doc == nil {
		return false, ""
	}

	phase, found, err := unstructured.NestedString(doc.Object, "status", "status")
	if err != nil {
		return false, ""
	}
	phase = strings.TrimSpace(phase)
	if !found || phase == "" {
		return true, ""
	}

	return isHealthyPhase(phase), phase
}

func isHealthyPhase(phase string) bool {
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		return true
	}

	switch phase {
	case "healthy", "ready", "running", "succeeded":
		return true
	}

	if strings.Contains(phase, "healthy") || strings.Contains(phase, "ready") {
		return true
	}

	return false
}
