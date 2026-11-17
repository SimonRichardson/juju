// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package ociworkload reconciles OCI images for machine-hosted units. It
// starts only after a unit has been assigned to a machine and uses that unit's
// API credentials to obtain image resources. Containerd itself is managed by
// the machine's provisioning configuration.
package ociworkload
