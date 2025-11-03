// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package plugin

import (
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/http"
	"github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	"github.com/cloudnative-pg/cnpg-i/pkg/operator"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/documentdb/cnpg-i-sidecar-injector/internal/identity"
	lifecycleImpl "github.com/documentdb/cnpg-i-sidecar-injector/internal/lifecycle"
	operatorImpl "github.com/documentdb/cnpg-i-sidecar-injector/internal/operator"
)

// NewCmd creates the `plugin` command
func NewCmd() *cobra.Command {
	cmd := http.CreateMainCmd(identity.Implementation{}, func(server *grpc.Server) error {
		// Register the declared implementations
		operator.RegisterOperatorServer(server, operatorImpl.Implementation{})
		lifecycle.RegisterOperatorLifecycleServer(server, lifecycleImpl.Implementation{})
		return nil
	})

	// If you want to provide your own logr.Logger here, inject it into a context.Context
	// with logr.NewContext(ctx, logger) and pass it to cmd.SetContext(ctx)
	log.SetLogger(zap.New(zap.UseDevMode(true)))

	// Additional custom behaviour can be added by wrapping cmd.PersistentPreRun or cmd.Run

	cmd.Use = "plugin"

	return cmd
}
