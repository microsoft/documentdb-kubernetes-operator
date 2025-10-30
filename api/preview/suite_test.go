// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package preview

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPIPreview(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Preview Suite")
}
