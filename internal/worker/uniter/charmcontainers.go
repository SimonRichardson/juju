// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/juju/juju/core/logger"
	charmmeta "github.com/juju/juju/domain/deployment/charm"
	"github.com/juju/juju/internal/worker/uniter/charm"
)

// CharmDirContainerNames returns the sorted names of the workload
// containers declared by the charm currently deployed in the supplied
// directory. A missing metadata file means no charm is deployed there yet,
// which is not an error.
func CharmDirContainerNames(charmDir string) ([]string, error) {
	meta, err := charmmeta.ReadCharmDirMetadata(charmDir)
	if errors.Is(err, charmmeta.FileNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading deployed charm metadata: %w", err)
	}
	names := make([]string, 0, len(meta.Containers))
	for name := range meta.Containers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// charmContainerDeployer wraps a charm deployer so the uniter learns the
// workload container names of every charm it deploys.
//
// It is used by unit agents whose container names follow the deployed
// charm: the workload hook resolvers follow the names at the next resolver
// rebuild, and the workload monitor replaces its pebble pollers and
// noticers for the new set.
type charmContainerDeployer struct {
	charm.Deployer

	charmDir string
	// charmContainers derives the container names from the charm deployed
	// in charmDir.
	charmContainers func(string) ([]string, error)
	// containers returns the container names the uniter is currently
	// wired for; the deployer reconciles them after each deploy.
	containers func() []string
	// onChange records a changed container set with the uniter and its
	// workload monitor.
	onChange func([]string)
	logger   logger.Logger
}

// Deploy deploys the staged charm, then reconciles the uniter's container
// names with the deployed charm metadata.
func (d *charmContainerDeployer) Deploy() error {
	if err := d.Deployer.Deploy(); err != nil {
		return err
	}
	names, err := d.charmContainers(d.charmDir)
	if err != nil {
		// The charm itself deployed; a metadata read failure must not
		// fail the operation. The next reconcile will pick the names up.
		d.logger.Warningf(context.Background(), "reading workload containers of deployed charm: %v", err)
		return nil
	}
	if !slices.Equal(d.containers(), names) {
		d.onChange(names)
	}
	return nil
}
