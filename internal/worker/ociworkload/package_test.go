// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload_test

//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload_test -destination workload_mock_test.go github.com/juju/juju/internal/worker/ociworkload Application,ResourceReader,ImagePuller,ContainerRuntime
//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload_test -destination guest_mock_test.go github.com/juju/juju/internal/worker/fortress Guest
//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload_test -destination dependency_mock_test.go github.com/juju/worker/v5/dependency Getter
//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload_test -destination agent_mock_test.go github.com/juju/juju/agent Agent,Config
//go:generate go run github.com/canonical/gomock/mockgen -package ociworkload_test -destination connection_mock_test.go github.com/juju/juju/api Connection
