// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package utils

import "encoding/json"

// GetKind gets the Kubernetes object kind from its JSON representation
func GetKind(definition []byte) (string, error) {
	var genericObject struct {
		Kind string `json:"kind"`
	}

	if err := json.Unmarshal(definition, &genericObject); err != nil {
		return "", err
	}

	return genericObject.Kind, nil
}
