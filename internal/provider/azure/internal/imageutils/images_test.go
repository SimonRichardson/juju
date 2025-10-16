// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package imageutils_test

import (
	"net/http"
	stdtesting "testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/arch"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/internal/provider/azure/internal/azuretesting"
	"github.com/juju/juju/internal/provider/azure/internal/imageutils"
	"github.com/juju/juju/internal/testing"
)

type imageutilsSuite struct {
	testing.BaseSuite

	mockSender *azuretesting.MockSender
	client     *armcompute.VirtualMachineImagesClient

	invalidator *MockCredentialInvalidator
}

func TestImageutilsSuite(t *stdtesting.T) {
	tc.Run(t, &imageutilsSuite{})
}

func (s *imageutilsSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)
	s.mockSender = &azuretesting.MockSender{}
	opts := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Transport: s.mockSender,
		},
	}
	var err error
	s.client, err = armcompute.NewVirtualMachineImagesClient("subscription-id", &azuretesting.FakeCredential{}, opts)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *imageutilsSuite) TestBaseImageOldStyleGen2(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "20_04-lts-gen2"}, {"name": "20_04-lts"}, {"name": "20_04-lts-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "20.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:0001-com-ubuntu-server-focal:20_04-lts-gen2:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageOldStyleARM64(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "20_04-lts-gen2"}, {"name": "20_04-lts"}, {"name": "20_04-lts-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "20.04"), "released", "westus", "arm64", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:0001-com-ubuntu-server-focal:20_04-lts-arm64:latest",
		Arch:     arch.ARM64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageOldStyleFallbackToGen1(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "20_04-lts-gen2"}, {"name": "20_04-lts"}, {"name": "20_04-lts-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "20.04"), "released", "westus", "", s.client, true)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:0001-com-ubuntu-server-focal:20_04-lts:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageOldStyleInvalidSKU(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "22_04-invalid"}, {"name": "22_04-lts"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "22.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:0001-com-ubuntu-server-jammy:22_04-lts:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageGen2(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "server"}, {"name": "server-gen1"}, {"name": "server-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "24.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:ubuntu-24_04-lts:server:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageFallbackToGen1(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "server"}, {"name": "server-gen1"}, {"name": "server-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "24.04"), "released", "westus", "", s.client, true)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:ubuntu-24_04-lts:server-gen1:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageARM64(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "server"}, {"name": "server-gen1"}, {"name": "server-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "24.04"), "released", "westus", "arm64", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:ubuntu-24_04-lts:server-arm64:latest",
		Arch:     arch.ARM64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageNonLTS(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "server"}, {"name": "server-gen1"}, {"name": "server-arm64"}]`,
	))
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "25.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image, tc.NotNil)
	c.Check(image, tc.DeepEquals, &instances.Image{
		Id:       "Canonical:ubuntu-25_04:server:latest",
		Arch:     arch.AMD64,
		VirtType: "Hyper-V",
	})
}

func (s *imageutilsSuite) TestBaseImageStreamDaily(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendAndRepeatResponse(azuretesting.NewResponseWithContent( //nolint:bodyclose
		`[{"name": "server"}, {"name": "minimal-gen1"}, {"name": "minimal-arm64"}, {"name": "minimal"}]`), 2)
	base := corebase.MakeDefaultBase("ubuntu", "24.04")

	image, err := imageutils.BaseImage(c.Context(), s.invalidator, base, "daily", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(image.Id, tc.Equals, "Canonical:ubuntu-24_04-lts-daily:server:latest")
}

func (s *imageutilsSuite) TestBaseImageOldStyleNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent(`[]`)) //nolint:bodyclose
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "22.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorMatches, `selecting SKU for ubuntu@22.04: legacy ubuntu "jammy" SKUs for released stream not found`)
	c.Check(image, tc.IsNil)
}

func (s *imageutilsSuite) TestBaseImageNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent(`[]`)) //nolint:bodyclose
	image, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "24.04"), "released", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorMatches, `selecting SKU for ubuntu@24.04: ubuntu "ubuntu@24.04/stable" SKUs for released stream not found`)
	c.Check(image, tc.IsNil)
}

func (s *imageutilsSuite) TestBaseImageStreamNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithContent(`[{"name": "22_04-beta1"}]`)) //nolint:bodyclose
	_, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "22.04"), "whatever", "westus", "", s.client, false)
	c.Assert(err, tc.ErrorMatches, `selecting SKU for ubuntu@22.04: legacy ubuntu "jammy" SKUs for whatever stream not found`)
}

func (s *imageutilsSuite) TestBaseImageStreamThrewCredentialError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithStatus("401 Unauthorized", http.StatusUnauthorized)) //nolint:bodyclose

	s.invalidator.EXPECT().InvalidateCredentials(gomock.Any(), gomock.Any()).Return(nil)

	_, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "22.04"), "whatever", "westus", "", s.client, false)
	c.Assert(err.Error(), tc.Contains, "RESPONSE 401")
}

func (s *imageutilsSuite) TestBaseImageStreamThrewNonCredentialError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.mockSender.AppendResponse(azuretesting.NewResponseWithStatus("308 Permanent Redirect", http.StatusPermanentRedirect)) //nolint:bodyclose

	_, err := imageutils.BaseImage(c.Context(), s.invalidator, corebase.MakeDefaultBase("ubuntu", "22.04"), "whatever", "westus", "", s.client, false)
	c.Assert(err.Error(), tc.Contains, "RESPONSE 308")
}

func (s *imageutilsSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.invalidator = NewMockCredentialInvalidator(ctrl)

	return ctrl
}
