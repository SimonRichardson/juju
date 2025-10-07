// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/collections/transform"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/trace"
	domainmachine "github.com/juju/juju/domain/machine"
	"github.com/juju/juju/internal/errors"
)

// GetInstanceID returns the cloud specific instance id for this machine.
// The following errors may be returned:
//   - [github.com/juju/juju/domain/machine/errors.NotProvisioned] if the machine
//     is not provisioned.
func (s *Service) GetInstanceID(ctx context.Context, machineUUID machine.UUID) (instance.Id, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	instanceId, err := s.st.GetInstanceID(ctx, machineUUID.String())
	if err != nil {
		return "", errors.Errorf("retrieving cloud instance id for machine %q: %w", machineUUID, err)
	}
	return instance.Id(instanceId), nil
}

// GetPollingInfos returns the polling information for the specified machines.
func (s *Service) GetPollingInfos(ctx context.Context, machineNames []machine.Name) (domainmachine.PollingInfos,
	error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	// If there are no machines, no need to reach the state.
	if len(machineNames) == 0 {
		return nil, nil
	}

	validationError := errors.Join(transform.Slice(machineNames, machine.Name.Validate)...)
	if validationError != nil {
		return nil, validationError
	}

	infos, err := s.st.GetPollingInfos(ctx, transform.Slice(machineNames, func(m machine.Name) string {
		return m.String()
	}))
	return infos, errors.Capture(err)
}

// GetInstanceIDAndName returns the cloud specific instance ID and display name for
// this machine.
// The following errors may be returned:
//   - [github.com/juju/juju/domain/machine/errors.NotProvisioned] if the machine
//     is not provisioned.
func (s *Service) GetInstanceIDAndName(ctx context.Context, machineUUID machine.UUID) (instance.Id, string, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	instanceID, instanceName, err := s.st.GetInstanceIDAndName(ctx, machineUUID.String())
	if err != nil {
		return "", "", errors.Errorf("retrieving cloud instance name for machine %q: %w", machineUUID, err)
	}
	return instance.Id(instanceID), instanceName, nil
}

// AvailabilityZone returns the availability zone for the specified machine.
func (s *Service) AvailabilityZone(ctx context.Context, machineUUID machine.UUID) (string, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	az, err := s.st.AvailabilityZone(ctx, machineUUID.String())
	if err != nil {
		return az, errors.Errorf("retrieving availability zone for machine %q: %w", machineUUID, err)
	}
	return az, nil
}

// GetHardwareCharacteristics returns the hardware characteristics of the
// of the specified machine.
func (s *Service) GetHardwareCharacteristics(ctx context.Context, machineUUID machine.UUID) (*instance.HardwareCharacteristics, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	hc, err := s.st.GetHardwareCharacteristics(ctx, machineUUID.String())
	if err != nil {
		return hc, errors.Errorf("retrieving hardware characteristics for machine %q: %w", machineUUID, err)
	}
	return hc, nil
}

// SetMachineCloudInstance sets an entry in the machine cloud instance table
// along with the instance tags and the link to a lxd profile if any.
func (s *Service) SetMachineCloudInstance(
	ctx context.Context,
	machineUUID machine.UUID,
	instanceID instance.Id,
	displayName, nonce string,
	hardwareCharacteristics *instance.HardwareCharacteristics,
) error {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	if err := s.st.SetMachineCloudInstance(ctx, machineUUID.String(), instanceID, displayName, nonce, hardwareCharacteristics); err != nil {
		return errors.Errorf("setting machine cloud instance for machine %q: %w", machineUUID, err)
	}
	return nil
}
