// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"errors"
	"fmt"
	"syscall"

	charmmeta "github.com/juju/juju/domain/deployment/charm"
)

const (
	// hookUserID is the uid and gid sidecar charm hooks run as when the
	// charm declares itself non-root.
	// NOTE: 170 uid/gid must be updated here and in
	// internal/provider/kubernetes/constants/constants.go and caas/Dockerfile.
	hookUserID = 170
	// hookSudoUserID is the uid and gid sidecar charm hooks run as when
	// the charm declares itself a sudoer.
	// NOTE: 171 uid/gid must be updated here and in
	// internal/provider/kubernetes/constants/constants.go.
	hookSudoUserID = 171
)

// CharmDirHookCredential returns the credential hook processes should run
// under for the charm deployed in the supplied directory, mirroring the
// security context Kubernetes applies to charm containers for the same
// charm-user. A nil credential means hooks keep running as the unit agent
// user: that covers machine charms without workload containers, which have
// never been governed by charm-user, and sidecar charms that leave it
// unset. A missing metadata file means no charm is deployed there yet,
// which is not an error.
func CharmDirHookCredential(charmDir string) (*syscall.Credential, error) {
	meta, err := charmmeta.ReadCharmDirMetadata(charmDir)
	if errors.Is(err, charmmeta.FileNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading deployed charm metadata: %w", err)
	}
	if len(meta.Containers) == 0 {
		return nil, nil
	}
	switch meta.CharmUser {
	case charmmeta.RunAsDefault:
		return nil, nil
	case charmmeta.RunAsRoot:
		return &syscall.Credential{Uid: 0, Gid: 0}, nil
	case charmmeta.RunAsSudoer:
		return &syscall.Credential{Uid: hookSudoUserID, Gid: hookSudoUserID}, nil
	case charmmeta.RunAsNonRoot:
		return &syscall.Credential{Uid: hookUserID, Gid: hookUserID}, nil
	default:
		return nil, fmt.Errorf("unknown charm-user %q expected one of %s, %s or %s",
			meta.CharmUser, charmmeta.RunAsRoot, charmmeta.RunAsSudoer, charmmeta.RunAsNonRoot)
	}
}
