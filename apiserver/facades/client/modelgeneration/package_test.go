// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

//go:generate go run github.com/canonical/gomock/mockgen -package modelgeneration -destination service_mock_test.go github.com/juju/juju/apiserver/facades/client/modelgeneration GenerationService,ApplicationService
