package process

import (
	_ "embed"
	"testing"
	"time"

	"github.com/DataDog/test-infra-definitions/components/datadog/agentparams"
	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/utils/e2e"
)

//go:embed config/process_check.yaml
var configStr string

type processTestSuite struct {
	e2e.Suite[e2e.FakeIntakeEnv]
}

func TestProcessTestSuite(t *testing.T) {
	e2e.Run(t, &processTestSuite{},
		e2e.FakeIntakeStackDef(e2e.WithAgentParams(agentparams.WithAgentConfig(configStr))))
}

func (s *processTestSuite) TestProcessCheck() {
	// force pulumi to deploy before running the test
	s.Env()

	s.EventuallyWithT(func(c *assert.CollectT) {
		payloads, err := s.Env().Fakeintake.GetProcesses()
		assert.NoError(c, err, "failed to get process payloads from fakeintake")
		assert.NotEmpty(c, payloads, "no process payloads returned")

		for _, payload := range payloads {
			s.T().Logf("payload!\n%v\n", payload)
		}
	}, 2*time.Minute, 10*time.Second)
}
