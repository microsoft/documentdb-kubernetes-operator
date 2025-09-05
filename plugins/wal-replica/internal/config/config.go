// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package config

import (
	"fmt"
	"strings"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/validation"
	"github.com/cloudnative-pg/cnpg-i/pkg/operator"
)

// Plugin parameter keys
const (
	ImageParam           = "image"           // string
	ReplicationHostParam = "replicationHost" // Required: primary host
	SynchronousParam     = "synchronous"     // enum: Active, Inactive, Unset
	WalDirectoryParam    = "walDirectory"    // directory where WAL is stored
)

// SynchronousMode represents the synchronous replication mode
type SynchronousMode string

const (
	SynchronousUnset    SynchronousMode = ""
	SynchronousActive   SynchronousMode = "active"
	SynchronousInactive SynchronousMode = "inactive"
)

const (
	defaultImage           = "ghcr.io/cloudnative-pg/postgresql:16"
	defaultWalDir          = "/var/lib/postgres/wal"
	defaultSynchronousMode = SynchronousInactive
)

// Configuration represents the plugin configuration parameters controlling the wal receiver pod
type Configuration struct {
	Image           string
	ReplicationHost string
	Synchronous     SynchronousMode
	WalDirectory    string
}

// FromParameters builds a plugin configuration from the configuration parameters
func FromParameters(helper *common.Plugin) *Configuration {
	cfg := &Configuration{}
	cfg.Image = helper.Parameters[ImageParam]
	cfg.ReplicationHost = helper.Parameters[ReplicationHostParam]
	cfg.Synchronous = SynchronousMode(strings.ToLower(helper.Parameters[SynchronousParam]))
	cfg.WalDirectory = helper.Parameters[WalDirectoryParam]
	return cfg
}

// ValidateChanges validates the changes between the old configuration to the new configuration
func ValidateChanges(_ *Configuration, _ *Configuration, _ *common.Plugin) []*operator.ValidationError {
	return nil
}

// ToParameters serialize the configuration back to plugin parameters
func (c *Configuration) ToParameters() (map[string]string, error) {
	params := map[string]string{}
	params[ImageParam] = c.Image
	params[ReplicationHostParam] = c.ReplicationHost
	params[SynchronousParam] = string(c.Synchronous)
	params[WalDirectoryParam] = c.WalDirectory
	return params, nil
}

// ValidateParams ensures that the provided parameters are valid
func ValidateParams(helper *common.Plugin) []*operator.ValidationError {
	validationErrors := make([]*operator.ValidationError, 0)

	// Must be present
	if raw, present := helper.Parameters[ReplicationHostParam]; !present || raw == "" {
		validationErrors = append(validationErrors, validation.BuildErrorForParameter(helper, ReplicationHostParam, "No replication host provided"))
	}

	// If present, must be valid
	if raw, present := helper.Parameters[SynchronousParam]; present && raw != "" {
		switch SynchronousMode(strings.ToLower(raw)) {
		case SynchronousActive, SynchronousInactive:
			// valid value
		default:
			validationErrors = append(validationErrors, validation.BuildErrorForParameter(helper, SynchronousParam,
				fmt.Sprintf("Invalid value '%s'. Must be 'active' or 'inactive'", raw)))
		}
	}
	return validationErrors
}

// applyDefaults fills the configuration with the defaults
// We know that replicationhost and sync are valid already
func (c *Configuration) ApplyDefaults() {
	if c.Image == "" {
		c.Image = defaultImage
	}
	if c.WalDirectory == "" {
		c.WalDirectory = defaultWalDir
	}
	if c.Synchronous == SynchronousUnset {
		c.Synchronous = defaultSynchronousMode
	}
}
