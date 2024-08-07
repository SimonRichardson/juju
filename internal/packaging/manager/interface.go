// Copyright 2015 Canonical Ltd.
// Copyright 2015 Cloudbase Solutions SRL
// Licensed under the LGPLv3, see LICENCE file for details.

package manager

import (
	"github.com/juju/proxy"
)

// PackageManager is the interface which carries out various
// package-management related work.
//
// TODO (stickupid): Both the PackageManager and PackageCommander from the
// commands package should be merged. The layout of the packaging package is
// over-engineered. The commands should be placed directly into the package
// types themselves and then the managers could be a lot more simpler.
type PackageManager interface {
	// InstallPrerequisite runs the command which installs the prerequisite
	// package which provides repository management functionalityes.
	InstallPrerequisite() error

	// Update runs the command to update the local package list.
	Update() error

	// Upgrade runs the command which issues an upgrade on all packages
	// with available newer versions.
	Upgrade() error

	// Install runs a *single* command that installs the given package(s).
	Install(packs ...string) error

	// Remove runs a *single* command that removes the given package(s).
	Remove(packs ...string) error

	// Purge runs the command that removes the given package(s) along
	// with any associated config files.
	Purge(packs ...string) error

	// Search runs the command that determines whether the given package is
	// available for installation from the currently configured repositories.
	Search(pack string) (bool, error)

	// IsInstalled runs the command which determines whether or not the
	// given package is currently installed on the system.
	IsInstalled(pack string) bool

	// AddRepository runs the command that adds a repository to the
	// list of available repositories.
	// NOTE: requires the prerequisite package whose installation command
	// is done by running InstallPrerequisite().
	AddRepository(repo string) error

	// RemoveRepository runs the command that removes a given
	// repository from the list of available repositories.
	// NOTE: requires the prerequisite package whose installation command
	// is done by running InstallPrerequisite().
	RemoveRepository(repo string) error

	// Cleanup runs the command that cleans up all orphaned packages,
	// left-over files and previously-cached packages.
	Cleanup() error

	// GetProxySettings returns the curretly-configured package manager proxy.
	GetProxySettings() (proxy.Settings, error)

	// SetProxy runs the commands to set the given proxy parameters for the
	// package management system.
	SetProxy(settings proxy.Settings) error
}
