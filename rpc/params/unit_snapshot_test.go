// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package params_test

import (
	"encoding/json"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/core/life"
	"github.com/juju/juju/rpc/params"
)

func TestUnitSnapshotSuite(t *stdtesting.T) {
	tc.Run(t, &unitSnapshotSuite{})
}

type unitSnapshotSuite struct{}

func (s *unitSnapshotSuite) TestJSONRoundTrip(c *tc.C) {
	privateAddress := "10.0.0.1"
	snapshot := params.UnitSnapshot{
		UnitName:             "postgresql/0",
		ApplicationName:      "postgresql",
		Life:                 life.Alive,
		ResolvedMode:         params.ResolvedRetryHooks,
		CharmURL:             "ch:postgresql-42",
		CharmModifiedVersion: 3,
		Leader:               true,
		Config:               map[string]any{"max-connections": float64(100)},
		Trust:                true,
		Relations: []params.RelationSnapshot{{
			ID:                        4,
			Name:                      "database",
			Endpoint:                  "database",
			Life:                      life.Alive,
			RemoteApplication:         "wordpress",
			MySettings:                map[string]string{"host": "10.0.0.1"},
			MyApplicationSettings:     map[string]string{"database": "postgresql"},
			RemoteUnits:               []params.RemoteUnitSnapshot{{Name: "wordpress/0", Settings: map[string]string{"ingress": "true"}}},
			RemoteApplicationSettings: map[string]string{"version": "1"},
		}},
		Storage: []params.StorageSnapshot{{
			ID:       "filesystem-0",
			Kind:     "filesystem",
			Location: "/var/lib/postgresql",
			Life:     life.Alive,
		}},
		Secrets: []params.SecretSnapshot{{
			URI:      "secret:abc",
			Label:    "database-password",
			Revision: 2,
			Value:    "secret-value",
		}},
		Addresses:         []string{"10.0.0.1"},
		PortRanges:        []params.PortRange{{FromPort: 5432, ToPort: 5432, Protocol: "tcp"}},
		UnitStatus:        params.DetailedStatus{Status: "active"},
		ApplicationStatus: params.DetailedStatus{Status: "active"},
		GoalState: params.GoalState{Units: params.UnitsGoalState{
			"postgresql/0": {Status: "started"},
		}},
		CharmState:          map[string]string{"initialized": "true"},
		WorkloadVersion:     "16.0",
		APIAddresses:        []string{"10.0.0.2:17070"},
		CloudAPIVersion:     "1.30",
		LegacyProxySettings: params.ProxySettings{HTTPProxy: "http://proxy.example"},
		JujuProxySettings:   params.ProxySettings{NoProxy: "10.0.0.0/8"},
		PrivateAddress:      &privateAddress,
		CharmTracingConfig:  &params.CharmTracingConfig{HTTPEndpoint: "https://trace.example"},
	}

	data, err := json.Marshal(snapshot)
	c.Assert(err, tc.ErrorIsNil)

	var decoded params.UnitSnapshot
	err = json.Unmarshal(data, &decoded)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(decoded, tc.DeepEquals, snapshot)
}

func (s *unitSnapshotSuite) TestOptionalFieldsAreOmitted(c *tc.C) {
	data, err := json.Marshal(params.UnitSnapshot{})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(string(data), tc.Not(tc.Contains), "resolved-mode")
	c.Check(string(data), tc.Not(tc.Contains), "private-address")
	c.Check(string(data), tc.Not(tc.Contains), "charm-tracing-config")
}
