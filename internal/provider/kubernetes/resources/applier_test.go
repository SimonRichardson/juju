// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package resources_test

import (
	"context"

	"github.com/juju/errors"
	jc "github.com/juju/testing/checkers"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juju/juju/internal/provider/kubernetes/resources"
	"github.com/juju/juju/internal/provider/kubernetes/resources/mocks"
	coretesting "github.com/juju/juju/testing"
)

type applierSuite struct {
	coretesting.BaseSuite
}

var _ = gc.Suite(&applierSuite{})

func (s *applierSuite) TestRun(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r2 := mocks.NewMockResource(ctrl)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.Apply(r1)
	applier.Delete(r2)

	gomock.InOrder(
		r1.EXPECT().Clone().Return(r1),
		r1.EXPECT().Get(gomock.Any()).Return(errors.NewNotFound(nil, "")),
		r1.EXPECT().Apply(gomock.Any()).Return(nil),
		r1.EXPECT().ID().Return(resources.ID{"A", "r1", "namespace"}),

		r2.EXPECT().Clone().Return(r2),
		r2.EXPECT().Get(gomock.Any()).Return(errors.NewNotFound(nil, "")),
		r2.EXPECT().Delete(gomock.Any()).Return(nil),
	)
	c.Assert(applier.Run(context.TODO(), false), jc.ErrorIsNil)
}

func (s *applierSuite) TestRunApplyFailedWithRollBackForNewResource(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r1Meta := &metav1.ObjectMeta{Name: "r1"}
	r1.EXPECT().GetObjectMeta().AnyTimes().Return(r1Meta)

	r2 := mocks.NewMockResource(ctrl)
	r2Meta := &metav1.ObjectMeta{Name: "r2"}
	r2.EXPECT().GetObjectMeta().AnyTimes().Return(r2Meta)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.Apply(r1)
	applier.Apply(r2)

	existingR1 := mocks.NewMockResource(ctrl)
	existingR1Meta := &metav1.ObjectMeta{}
	existingR1.EXPECT().GetObjectMeta().AnyTimes().Return(existingR1Meta)

	existingR2 := mocks.NewMockResource(ctrl)
	existingR2Meta := &metav1.ObjectMeta{}
	existingR2.EXPECT().GetObjectMeta().AnyTimes().Return(existingR2Meta)

	gomock.InOrder(
		r1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(errors.NewNotFound(nil, "")),
		r1.EXPECT().Apply(gomock.Any()).Return(nil),
		r1.EXPECT().ID().Return(resources.ID{"A", "r1", "namespace"}),

		r2.EXPECT().Clone().Return(existingR2),
		existingR2.EXPECT().Get(gomock.Any()).Return(nil),
		r2.EXPECT().Apply(gomock.Any()).Return(errors.New("something was wrong")),
		r2.EXPECT().ID().Return(resources.ID{"B", "r2", "namespace"}),

		// Rollback only r1 because r2 apply failed.
		r1.EXPECT().Clone().Return(r1),
		r1.EXPECT().Get(gomock.Any()).Return(nil),

		// Delete the new resource r1 that was just created.
		r1.EXPECT().Delete(gomock.Any()).Return(nil),
	)
	c.Assert(applier.Run(context.TODO(), false),
		gc.ErrorMatches, `applying resource "r2": something was wrong`)
}

func (s *applierSuite) TestRunApplyResourceVersionChanged(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r1Meta := &metav1.ObjectMeta{
		ResourceVersion: "1",
	}
	r1.EXPECT().ID().AnyTimes().Return(resources.ID{"A", "r1", "namespace"})
	r1.EXPECT().GetObjectMeta().AnyTimes().Return(r1Meta)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.Apply(r1)

	existingR1 := mocks.NewMockResource(ctrl)
	existingR1Meta := &metav1.ObjectMeta{
		ResourceVersion: "2",
	}
	existingR1.EXPECT().GetObjectMeta().AnyTimes().Return(existingR1Meta)

	gomock.InOrder(
		r1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(nil),
		r1.EXPECT().Apply(gomock.Any()).Return(errors.New("resource version conflict")),
	)
	c.Assert(applier.Run(context.TODO(), false), gc.ErrorMatches, `applying resource "r1": resource version conflict`)
}

func (s *applierSuite) TestRunApplyFailedWithRollBackForExistingResource(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r1Meta := &metav1.ObjectMeta{Name: "r1"}
	r1.EXPECT().GetObjectMeta().AnyTimes().Return(r1Meta)

	r2 := mocks.NewMockResource(ctrl)
	r2Meta := &metav1.ObjectMeta{Name: "r2"}
	r2.EXPECT().GetObjectMeta().AnyTimes().Return(r2Meta)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.Apply(r1)
	applier.Apply(r2)

	existingR1 := mocks.NewMockResource(ctrl)
	existingR1Meta := &metav1.ObjectMeta{}
	existingR1.EXPECT().GetObjectMeta().AnyTimes().Return(existingR1Meta)

	existingR2 := mocks.NewMockResource(ctrl)
	existingR2Meta := &metav1.ObjectMeta{}
	existingR2.EXPECT().GetObjectMeta().AnyTimes().Return(existingR2Meta)

	gomock.InOrder(
		r1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(nil),
		r1.EXPECT().Apply(gomock.Any()).Return(nil),
		r1.EXPECT().ID().Return(resources.ID{"A", "r1", "namespace"}),

		r2.EXPECT().Clone().Return(existingR2),
		existingR2.EXPECT().Get(gomock.Any()).Return(nil),
		r2.EXPECT().Apply(gomock.Any()).Return(errors.New("something was wrong")),
		r2.EXPECT().ID().Return(resources.ID{"A", "r2", "namespace"}),

		// Rollback only r1 because r2 apply failed.
		existingR1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(nil),
		// Re-apply the old successfully applied resource.
		existingR1.EXPECT().Apply(gomock.Any()).Return(nil),
		existingR1.EXPECT().ID().Return(resources.ID{"B", "existingr1", "namespace"}),
	)
	c.Assert(applier.Run(context.TODO(), false),
		gc.ErrorMatches, `applying resource "r2": something was wrong`)
}

func (s *applierSuite) TestRunDeleteFailedWithRollBack(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r1Meta := &metav1.ObjectMeta{Name: "r1"}
	r1.EXPECT().GetObjectMeta().AnyTimes().Return(r1Meta)

	r2 := mocks.NewMockResource(ctrl)
	r2Meta := &metav1.ObjectMeta{Name: "r2"}
	r2.EXPECT().GetObjectMeta().AnyTimes().Return(r2Meta)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.Delete(r1)
	applier.Delete(r2)

	existingR1 := mocks.NewMockResource(ctrl)
	existingR1Meta := &metav1.ObjectMeta{}
	existingR1.EXPECT().GetObjectMeta().AnyTimes().Return(existingR1Meta)

	existingR2 := mocks.NewMockResource(ctrl)
	existingR2Meta := &metav1.ObjectMeta{}
	existingR2.EXPECT().GetObjectMeta().AnyTimes().Return(existingR2Meta)

	gomock.InOrder(
		r1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(nil),
		r1.EXPECT().Delete(gomock.Any()).Return(nil),

		r2.EXPECT().Clone().Return(existingR2),
		existingR2.EXPECT().Get(gomock.Any()).Return(nil),
		r2.EXPECT().Delete(gomock.Any()).Return(errors.New("something was wrong")),
		r2.EXPECT().ID().Return(resources.ID{"B", "r2", "namespace"}),

		// Rollback only r1 because r2 delete failed.
		existingR1.EXPECT().Clone().Return(existingR1),
		existingR1.EXPECT().Get(gomock.Any()).Return(nil),
		// Re-apply the old resource.
		existingR1.EXPECT().Apply(gomock.Any()).Return(nil),
		existingR1.EXPECT().ID().Return(resources.ID{"A", "existingr1", "namespace"}),
	)
	c.Assert(applier.Run(context.TODO(), false), gc.ErrorMatches, `deleting resource "r2": something was wrong`)
}

func (s *applierSuite) TestApplySet(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	r1 := mocks.NewMockResource(ctrl)
	r1.EXPECT().ID().AnyTimes().Return(resources.ID{"A", "r1", "namespace"})
	r1Meta := &metav1.ObjectMeta{}
	r1.EXPECT().GetObjectMeta().AnyTimes().Return(r1Meta)
	r1.EXPECT().Clone().AnyTimes().Return(r1)
	r2 := mocks.NewMockResource(ctrl)
	r2.EXPECT().ID().AnyTimes().Return(resources.ID{"B", "r2", "namespace"})
	r2Meta := &metav1.ObjectMeta{}
	r2.EXPECT().GetObjectMeta().AnyTimes().Return(r2Meta)
	r2.EXPECT().Clone().AnyTimes().Return(r2)
	r2Copy := mocks.NewMockResource(ctrl)
	r2Copy.EXPECT().ID().AnyTimes().Return(resources.ID{"B", "r2", "namespace"})
	r2CopyMeta := &metav1.ObjectMeta{}
	r2Copy.EXPECT().GetObjectMeta().AnyTimes().Return(r2CopyMeta)
	r2Copy.EXPECT().Clone().AnyTimes().Return(r2)
	r3 := mocks.NewMockResource(ctrl)
	r3.EXPECT().ID().AnyTimes().Return(resources.ID{"A", "r3", "namespace"})
	r3Meta := &metav1.ObjectMeta{}
	r3.EXPECT().GetObjectMeta().AnyTimes().Return(r3Meta)
	r3.EXPECT().Clone().AnyTimes().Return(r3)

	applier := resources.NewApplierForTest()
	c.Assert(len(applier.Operations()), gc.DeepEquals, 0)
	applier.ApplySet([]resources.Resource{r1, r2}, []resources.Resource{r2Copy, r3})

	gomock.InOrder(
		r1.EXPECT().Get(gomock.Any()).Return(nil),
		r1.EXPECT().Delete(gomock.Any()).Return(nil),
		r2.EXPECT().Get(gomock.Any()).Return(nil),
		r2Copy.EXPECT().Apply(gomock.Any()).Return(nil),
		r3.EXPECT().Get(gomock.Any()).Return(errors.NotFoundf("missing aye")),
		r3.EXPECT().Apply(gomock.Any()).Return(nil),
	)
	c.Assert(applier.Run(context.TODO(), false), jc.ErrorIsNil)
}
