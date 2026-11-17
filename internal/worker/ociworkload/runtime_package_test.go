// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload -destination runtime_client_mock_test.go github.com/juju/juju/internal/worker/ociworkload ContainerdClient
//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload -destination runtime_task_mock_test.go github.com/containerd/containerd/v2/client Container,Task,Image
