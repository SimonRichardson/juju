// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package azure_test

import (
	"context"
	"net/http"
	stdtesting "testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/juju/tc"

	"github.com/juju/juju/environs"
	"github.com/juju/juju/internal/provider/azure"
	"github.com/juju/juju/internal/provider/azure/internal/azuretesting"
	"github.com/juju/juju/internal/testing"
)

type environUpgradeSuite struct {
	testing.BaseSuite

	requests []*http.Request
	sender   azuretesting.Senders
	provider environs.EnvironProvider
	env      environs.Environ

	credentialInvalidator environs.CredentialInvalidator
	invalidatedCredential bool
}

func TestEnvironUpgradeSuite(t *stdtesting.T) {
	tc.Run(t, &environUpgradeSuite{})
}

func (s *environUpgradeSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)
	s.sender = nil
	s.requests = nil

	s.provider = newProvider(c, azure.ProviderConfig{
		Sender:           azuretesting.NewSerialSender(&s.sender),
		RequestInspector: &azuretesting.RequestRecorderPolicy{Requests: &s.requests},
		CreateTokenCredential: func(appId, appPassword, tenantID string, opts azcore.ClientOptions) (azcore.TokenCredential, error) {
			return &azuretesting.FakeCredential{}, nil
		},
	})
	s.env = openEnviron(c, s.provider, s.credentialInvalidator, &s.sender)
	s.requests = nil

	s.invalidatedCredential = false
	s.credentialInvalidator = azure.CredentialInvalidator(func(context.Context, environs.CredentialInvalidReason) error {
		s.invalidatedCredential = true
		return nil
	})
}

func (s *environUpgradeSuite) TestEnvironImplementsUpgrader(c *tc.C) {
	c.Assert(s.env, tc.Implements, new(environs.Upgrader))
}

func (s *environUpgradeSuite) TestEnvironUpgradeOperations(c *tc.C) {
	upgrader := s.env.(environs.Upgrader)
	ops := upgrader.UpgradeOperations(c.Context(), environs.UpgradeOperationsParams{})
	c.Assert(ops, tc.HasLen, 1)
	c.Assert(ops[0].TargetVersion, tc.Equals, 1)
	c.Assert(ops[0].Steps, tc.HasLen, 1)
	c.Assert(ops[0].Steps[0].Description(), tc.Equals, "Create common resource deployment")
}

func (s *environUpgradeSuite) TestEnvironUpgradeOperationCreateCommonDeployment(c *tc.C) {
	upgrader := s.env.(environs.Upgrader)
	op0 := upgrader.UpgradeOperations(c.Context(), environs.UpgradeOperationsParams{})[0]

	// The existing NSG has two rules: one for Juju API traffic,
	// and an application-specific rule. Only the latter should
	// be preserved; we will recreate the "builtin" SSH rule,
	// and the API rule is not needed for non-controller models.
	customRule := &armnetwork.SecurityRule{
		Name: to.Ptr("machine-0-tcp-1234"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:              to.Ptr("custom rule"),
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("1234"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(102)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
		},
	}
	securityRules := []*armnetwork.SecurityRule{{
		Name: to.Ptr("JujuAPIInbound"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:              to.Ptr("Allow API connections to controller machines"),
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("192.168.16.0/20"),
			DestinationPortRange:     to.Ptr("17777"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(101)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
		},
	}, customRule}
	nsg := armnetwork.SecurityGroup{
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRules,
		},
	}

	vmListSender := azuretesting.NewSenderWithValue(&armcompute.VirtualMachineListResult{})
	vmListSender.PathPattern = ".*/virtualMachines"
	nsgSender := azuretesting.NewSenderWithValue(&nsg)
	nsgSender.PathPattern = ".*/networkSecurityGroups/juju-internal-nsg"
	deploymentSender := azuretesting.NewSenderWithValue(&armresources.Deployment{})
	deploymentSender.PathPattern = ".*/deployments/common"
	s.sender = append(s.sender, vmListSender, nsgSender, deploymentSender)
	c.Assert(op0.Steps[0].Run(c.Context()), tc.ErrorIsNil)
	c.Assert(s.requests, tc.HasLen, 3)

	var actual armresources.Deployment
	unmarshalRequestBody(c, s.requests[2], &actual)
	c.Assert(actual.Properties, tc.NotNil)
	c.Assert(actual.Properties.Template, tc.NotNil)
	resources, ok := actual.Properties.Template.(map[string]interface{})["resources"].([]interface{})
	c.Assert(ok, tc.IsTrue)
	c.Assert(resources, tc.HasLen, 2)
}

func (s *environUpgradeSuite) TestEnvironUpgradeOperationCreateCommonDeploymentControllerModel(c *tc.C) {
	s.sender = nil
	env := openEnviron(c, s.provider, s.credentialInvalidator, &s.sender, testing.Attrs{"name": "controller"})
	s.requests = nil
	upgrader := env.(environs.Upgrader)

	controllerTags := make(map[string]*string)
	trueString := "true"
	controllerTags["juju-is-controller"] = &trueString
	vms := []*armcompute.VirtualMachine{{
		Tags: nil,
	}, {
		Tags: controllerTags,
	}}
	vmListSender := azuretesting.NewSenderWithValue(&armcompute.VirtualMachineListResult{
		Value: vms,
	})
	vmListSender.PathPattern = ".*/virtualMachines"
	s.sender = append(s.sender, vmListSender)

	op0 := upgrader.UpgradeOperations(c.Context(), environs.UpgradeOperationsParams{})[0]
	c.Assert(op0.Steps[0].Run(c.Context()), tc.ErrorIsNil)
}

func (s *environUpgradeSuite) TestEnvironUpgradeOperationCreateCommonDeploymentControllerModelWithInvalidCredential(c *tc.C) {
	s.sender = nil
	s.requests = nil
	env := openEnviron(c, s.provider, s.credentialInvalidator, &s.sender, testing.Attrs{"name": "controller"})
	upgrader := env.(environs.Upgrader)

	controllerTags := make(map[string]*string)
	trueString := "true"
	controllerTags["juju-is-controller"] = &trueString

	unauthSender := &azuretesting.MockSender{}
	unauthSender.AppendAndRepeatResponse(azuretesting.NewResponseWithStatus("401 Unauthorized", http.StatusUnauthorized), 3) //nolint:bodyclose
	s.sender = append(s.sender, unauthSender, unauthSender, unauthSender)

	c.Assert(s.invalidatedCredential, tc.IsFalse)
	op0 := upgrader.UpgradeOperations(c.Context(), environs.UpgradeOperationsParams{})[0]
	c.Assert(op0.Steps[0].Run(c.Context()), tc.NotNil)
	c.Assert(s.invalidatedCredential, tc.IsTrue)
}
