// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charm_test

//go:generate go run go.uber.org/mock/mockgen -typed -package charm -destination charm_mock_test.go github.com/juju/juju/internal/charm CharmMeta
//go:generate go run go.uber.org/mock/mockgen -typed -package charm -destination core_charm_mock_test.go github.com/juju/juju/core/charm SelectorModelConfig
