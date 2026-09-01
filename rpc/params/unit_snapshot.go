// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package params

import "github.com/juju/juju/core/life"

// UnitSnapshot is the complete state delivered to a holistic unit runtime.
// It deliberately has no event reason: the charm reconciles from the current
// state on every dispatch.
type UnitSnapshot struct {
	UnitName        string `json:"unit-name"`
	ApplicationName string `json:"application-name"`

	Life         life.Value   `json:"life"`
	ResolvedMode ResolvedMode `json:"resolved-mode,omitempty"`

	CharmURL             string `json:"charm-url"`
	CharmModifiedVersion int    `json:"charm-modified-version"`

	Leader bool `json:"leader"`

	Config map[string]any `json:"config"`
	Trust  bool           `json:"trust"`

	Relations []RelationSnapshot `json:"relations"`
	Storage   []StorageSnapshot  `json:"storage"`
	Secrets   []SecretSnapshot   `json:"secrets"`

	Addresses  []string    `json:"addresses"`
	PortRanges []PortRange `json:"port-ranges"`

	UnitStatus        DetailedStatus    `json:"unit-status"`
	ApplicationStatus DetailedStatus    `json:"application-status"`
	GoalState         GoalState         `json:"goal-state"`
	CharmState        map[string]string `json:"charm-state"`
	WorkloadVersion   string            `json:"workload-version"`

	APIAddresses        []string            `json:"api-addresses"`
	CloudAPIVersion     string              `json:"cloud-api-version"`
	LegacyProxySettings ProxySettings       `json:"legacy-proxy-settings"`
	JujuProxySettings   ProxySettings       `json:"juju-proxy-settings"`
	PrivateAddress      *string             `json:"private-address,omitempty"`
	CharmTracingConfig  *CharmTracingConfig `json:"charm-tracing-config,omitempty"`
}

// RelationSnapshot is the complete state of one relation from the local
// unit's perspective.
type RelationSnapshot struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Endpoint  string     `json:"endpoint"`
	Life      life.Value `json:"life"`
	Suspended bool       `json:"suspended"`

	RemoteApplication string `json:"remote-application"`

	MySettings                map[string]string    `json:"my-settings"`
	MyApplicationSettings     map[string]string    `json:"my-application-settings"`
	RemoteUnits               []RemoteUnitSnapshot `json:"remote-units"`
	RemoteApplicationSettings map[string]string    `json:"remote-application-settings,omitempty"`
}

// RemoteUnitSnapshot is the relation data for a remote unit.
type RemoteUnitSnapshot struct {
	Name     string            `json:"name"`
	Settings map[string]string `json:"settings"`
}

// StorageSnapshot is the state of one storage attachment on the unit.
type StorageSnapshot struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Location string     `json:"location"`
	Life     life.Value `json:"life"`
}

// SecretSnapshot identifies the revision of a secret visible to the unit.
//
// It deliberately excludes secret content and backend references. A charm
// retrieves secret content through the regular secret API after reconciling.
type SecretSnapshot struct {
	URI      string `json:"uri"`
	Label    string `json:"label,omitempty"`
	Revision int    `json:"revision"`
}
