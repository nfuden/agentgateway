//go:build e2e

package tls

import (
	"context"
	"net/http"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"

	"github.com/agentgateway/agentgateway/controller/pkg/utils/kubeutils"
	"github.com/agentgateway/agentgateway/controller/pkg/utils/requestutils/curl"
	"github.com/agentgateway/agentgateway/controller/test/e2e"
	testdefaults "github.com/agentgateway/agentgateway/controller/test/e2e/defaults"
	"github.com/agentgateway/agentgateway/controller/test/e2e/tests/base"
	"github.com/agentgateway/agentgateway/controller/test/gomega/matchers"
)

var _ e2e.NewSuiteFunc = NewTestingSuite

type testingSuite struct {
	*base.BaseTestingSuite
}

func NewTestingSuite(ctx context.Context, testInst *e2e.TestInstallation) suite.TestingSuite {
	setup := base.TestCase{
		Manifests: []string{},
	}
	testCases := map[string]*base.TestCase{
		"TestTLSPlaintextModeBasicFunctionality": {
			Manifests: []string{
				basicGatewayManifest,
			},
		},
	}
	return &testingSuite{
		base.NewBaseTestingSuite(ctx, testInst, setup, testCases),
	}
}

// TestTLSPlaintextModeBasicFunctionality validates that the control plane with plaintext xDS mode
// can successfully configure a basic Gateway and route traffic.
func (s *testingSuite) TestTLSPlaintextModeBasicFunctionality() {
	s.TestInstallation.AssertionsT(s.T()).AssertEventualCurlResponse(
		s.Ctx,
		testdefaults.CurlPodExecOpt,
		[]curl.Option{
			curl.WithHost(kubeutils.ServiceFQDN(gateway.ObjectMeta)),
			curl.WithHostHeader("test.example.com"),
			curl.WithPort(8080),
			curl.WithPath("/headers"),
			curl.WithScheme("http"),
		},
		&matchers.HttpResponse{
			StatusCode: http.StatusOK,
			Body:       gomega.ContainSubstring("test.example.com"),
		},
	)
}
