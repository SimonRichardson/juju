// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	stdcontext "context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/juju/tc"

	internallogger "github.com/juju/juju/internal/logger"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/uniter/charm"
)

var errFailedDeploy = stderrors.New("deploy failed")

type charmContainersSuite struct {
	coretesting.BaseSuite
}

func TestCharmContainersSuite(t *testing.T) {
	tc.Run(t, &charmContainersSuite{})
}

const metadataWithContainers = `
name: redis-k8s
summary: Redis
description: Redis
containers:
  redis:
    resource: redis-image
resources:
  redis-image:
    type: oci-image
    description: Redis image
`

const metadataWithoutContainers = `
name: mysql
summary: MySQL
description: MySQL
`

func writeCharmMetadata(c *tc.C, charmDir, metadata string) {
	c.Assert(os.MkdirAll(charmDir, 0755), tc.ErrorIsNil)
	c.Assert(os.WriteFile(filepath.Join(charmDir, "metadata.yaml"), []byte(metadata), 0644), tc.ErrorIsNil)
}

// TestCharmDirContainerNames covers the container name derivation for
// undeployed charms, ordinary machine charms and sidecar charms.
func (s *charmContainersSuite) TestCharmDirContainerNames(c *tc.C) {
	charmDir := c.MkDir()

	// No charm deployed yet: no names and no error.
	names, err := CharmDirContainerNames(charmDir)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.HasLen, 0)

	writeCharmMetadata(c, charmDir, metadataWithoutContainers)
	names, err = CharmDirContainerNames(charmDir)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.HasLen, 0)

	writeCharmMetadata(c, charmDir, metadataWithContainers)
	names, err = CharmDirContainerNames(charmDir)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.DeepEquals, []string{"redis"})
}

// TestDeployerAdoptsChangedContainers checks that a deployment changing the
// container set notifies the uniter, while a deployment leaving it alone does
// not.
func (s *charmContainersSuite) TestDeployerAdoptsChangedContainers(c *tc.C) {
	charmDir := c.MkDir()
	writeCharmMetadata(c, charmDir, metadataWithoutContainers)

	current := []string{}
	var recorded [][]string
	deployer := &charmContainerDeployer{
		Deployer:        stubDeployer{},
		charmDir:        charmDir,
		charmContainers: CharmDirContainerNames,
		containers:      func() []string { return current },
		onChange: func(names []string) {
			current = names
			recorded = append(recorded, names)
		},
		logger: internallogger.Noop(),
	}

	// Deploying a charm that declares no containers changes nothing.
	c.Assert(deployer.Deploy(), tc.ErrorIsNil)
	c.Check(recorded, tc.HasLen, 0)

	// A charm declaring a workload container is adopted on deploy.
	writeCharmMetadata(c, charmDir, metadataWithContainers)
	c.Assert(deployer.Deploy(), tc.ErrorIsNil)
	c.Check(recorded, tc.DeepEquals, [][]string{{"redis"}})

	// Re-deploying the same charm keeps the set, without another change.
	c.Assert(deployer.Deploy(), tc.ErrorIsNil)
	c.Check(recorded, tc.HasLen, 1)
}

// TestDeployerPropagatesDeployError checks that a failed deployment is
// reported and does not touch the container set.
func (s *charmContainersSuite) TestDeployerPropagatesDeployError(c *tc.C) {
	deployer := &charmContainerDeployer{
		Deployer:        stubDeployer{err: errFailedDeploy},
		charmDir:        c.MkDir(),
		charmContainers: CharmDirContainerNames,
		containers:      func() []string { return nil },
		onChange:        func([]string) { c.Fatal("onChange must not run for a failed deploy") },
		logger:          internallogger.Noop(),
	}

	c.Check(deployer.Deploy(), tc.ErrorIs, errFailedDeploy)
}

// TestDeployerToleratesMetadataFailure checks a charm deploys even when its
// container metadata cannot be read back; the next deploy reconciles.
func (s *charmContainersSuite) TestDeployerToleratesMetadataFailure(c *tc.C) {
	deployer := &charmContainerDeployer{
		Deployer: stubDeployer{},
		charmDir: filepath.Join(c.MkDir(), "no", "such", "charm"),
		charmContainers: func(string) ([]string, error) {
			return nil, errFailedDeploy
		},
		containers: func() []string { return nil },
		onChange:   func([]string) { c.Fatal("onChange must not run for unreadable metadata") },
		logger:     internallogger.Noop(),
	}

	c.Assert(deployer.Deploy(), tc.ErrorIsNil)
}

type stubDeployer struct {
	err error
}

func (s stubDeployer) Stage(_ stdcontext.Context, _ charm.BundleInfo) error {
	return s.err
}

func (s stubDeployer) Deploy() error {
	return s.err
}
