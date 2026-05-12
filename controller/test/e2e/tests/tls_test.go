//go:build e2e

package tests_test

import (
	"context"
	"os"
	"testing"

	"github.com/agentgateway/agentgateway/controller/pkg/utils/envutils"
	"github.com/agentgateway/agentgateway/controller/test/e2e"
	. "github.com/agentgateway/agentgateway/controller/test/e2e/tests"
	"github.com/agentgateway/agentgateway/controller/test/e2e/testutils/install"
	"github.com/agentgateway/agentgateway/controller/test/testutils"
)

// TestControlPlaneTLS tests the control plane with plaintext xDS mode.
// This verifies that the controller can come up and serve basic traffic when
// xDS TLS is explicitly disabled.
func TestControlPlaneTLS(t *testing.T) {
	ctx := context.Background()
	installNs, nsEnvPredefined := envutils.LookupOrDefault(testutils.InstallNamespace, "agentgateway-tls-plaintext-test")

	testInstallation := e2e.CreateTestInstallation(
		t,
		&install.Context{
			InstallNamespace:          installNs,
			ProfileValuesManifestFile: e2e.EmptyValuesManifestPath,
			ValuesManifestFile:        e2e.ControlPlaneTLSPlaintextManifestPath,
			ExtraHelmArgs: []string{
				"--set", "controller.extraEnv.KGW_GLOBAL_POLICY_NAMESPACE=" + installNs,
			},
		},
	)

	if !nsEnvPredefined {
		os.Setenv(testutils.InstallNamespace, installNs)
	}

	// We register the cleanup function _before_ we actually perform the installation.
	// This allows us to uninstall agentgateway, in case the original installation only completed partially
	testutils.Cleanup(t, func() {
		if !nsEnvPredefined {
			os.Unsetenv(testutils.InstallNamespace)
		}
		if t.Failed() {
			testInstallation.PreFailHandler(ctx, t)
		}

		testInstallation.Uninstall(ctx, t)
	})

	// Install agentgateway with plaintext xDS mode
	testInstallation.InstallFromLocalChart(t.Context(), t)

	TLSSuiteRunner().Run(t.Context(), t, testInstallation)
}
