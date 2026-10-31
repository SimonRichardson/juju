// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/canonical/gomock/gomock"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/juju/tc"

	coretesting "github.com/juju/juju/internal/testing"
)

type runtimeSuite struct {
	coretesting.BaseSuite
}

// TestRuntimeSuite tests containerd task adoption, replacement, and cleanup.
func TestRuntimeSuite(t *testing.T) {
	tc.Run(t, &runtimeSuite{})
}

type runtimeFixture struct {
	spec    ContainerSpec
	runtime ContainerdRuntime
	client  *MockContainerdClient
	image   *MockImage
	ctrl    *gomock.Controller
}

func newRuntimeFixture(c *tc.C) *runtimeFixture {
	dir := c.MkDir()
	pebble := filepath.Join(dir, "pebble")
	c.Assert(os.WriteFile(pebble, []byte("pebble"), 0755), tc.ErrorIsNil)

	ctrl := gomock.NewController(c)
	daemon := NewMockContainerdClient(ctrl)
	image := NewMockImage(ctrl)
	fixture := &runtimeFixture{
		ctrl:   ctrl,
		client: daemon,
		image:  image,
		spec: ContainerSpec{
			UnitName:      "redis/0",
			Name:          "redis",
			Image:         "docker.io/library/redis:7",
			PebbleBinary:  pebble,
			ContainersDir: filepath.Join(dir, "containers"),
			StateDir:      filepath.Join(dir, "oci"),
		},
	}
	fixture.runtime = ContainerdRuntime{NewClient: func() (ContainerdClient, error) {
		return daemon, nil
	}}
	return fixture
}

func (f *runtimeFixture) expectImage() map[string]string {
	imageDigest := digest.FromString("redis image")
	f.client.EXPECT().GetImage(gomock.Any(), f.spec.Image).Return(f.image, nil)
	f.image.EXPECT().Target().Return(ocispec.Descriptor{Digest: imageDigest})
	f.client.EXPECT().Close().Return(nil)
	labels, err := workloadLabels(f.spec, imageDigest.String())
	if err != nil {
		panic(err)
	}
	return labels
}

// TestEnsureAdoptsRunningTask checks that a restarted worker leaves the
// matching workload untouched while retaining unit ownership.
func (s *runtimeSuite) TestEnsureAdoptsRunningTask(c *tc.C) {
	f := newRuntimeFixture(c)
	labels := f.expectImage()
	container := NewMockContainer(f.ctrl)
	task := NewMockTask(f.ctrl)
	f.client.EXPECT().LoadContainer(gomock.Any(), workloadID(f.spec.UnitName, f.spec.Name)).Return(container, nil)
	container.EXPECT().Labels(gomock.Any()).Return(labels, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(task, nil)
	task.EXPECT().Status(gomock.Any()).Return(client.Status{Status: client.Running}, nil)

	c.Assert(f.runtime.Ensure(c.Context(), f.spec), tc.ErrorIsNil)
	info, err := os.Stat(filepath.Join(f.spec.ContainersDir, f.spec.Name))
	c.Assert(err, tc.ErrorIsNil)
	c.Check(info.Mode().Perm(), tc.Equals, os.FileMode(0777))
	info, err = os.Stat(f.spec.ContainersDir)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(info.Mode().Perm(), tc.Equals, os.FileMode(0700))
}

// TestEnsureStartsMissingTask checks that an existing container without a
// running Pebble task is repaired without discarding its persistent plan.
func (s *runtimeSuite) TestEnsureStartsMissingTask(c *tc.C) {
	f := newRuntimeFixture(c)
	labels := f.expectImage()
	container := NewMockContainer(f.ctrl)
	newTask := NewMockTask(f.ctrl)
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(container, nil)
	container.EXPECT().Labels(gomock.Any()).Return(labels, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound).Times(2)
	container.EXPECT().NewTask(gomock.Any(), gomock.Any()).Return(newTask, nil)
	newTask.EXPECT().Start(gomock.Any()).Return(nil)

	c.Assert(f.runtime.Ensure(c.Context(), f.spec), tc.ErrorIsNil)
}

// TestEnsureRefusesForeignContainer checks that a workload ID collision
// cannot stop or replace another unit's task.
func (s *runtimeSuite) TestEnsureRefusesForeignContainer(c *tc.C) {
	f := newRuntimeFixture(c)
	f.expectImage()
	container := NewMockContainer(f.ctrl)
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(container, nil)
	container.EXPECT().Labels(gomock.Any()).Return(map[string]string{unitLabel: "other/0"}, nil)

	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `workload container .* belongs to another unit`)
}

// TestEnsureMissingPebbleFailsBeforeConnecting checks that an incomplete
// agent-tools archive cannot leave behind a container or an open connection.
func (s *runtimeSuite) TestEnsureMissingPebbleFailsBeforeConnecting(c *tc.C) {
	f := newRuntimeFixture(c)
	f.spec.PebbleBinary = filepath.Join(c.MkDir(), "absent")
	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `accessing Pebble binary: .*`)
}

// TestEnsureRejectsTraversal checks that container metadata cannot select a
// host path outside its assigned unit's socket and state directories.
func (s *runtimeSuite) TestEnsureRejectsTraversal(c *tc.C) {
	f := newRuntimeFixture(c)
	for _, name := range []string{"", ".", "..", "../redis", "a/b"} {
		f.spec.Name = name
		c.Logf("invalid container name: %q", name)
		c.Check(f.runtime.Ensure(c.Context(), f.spec), tc.NotNil)
	}
}

// TestRemoveUnitOnlyTouchesOwnedTasks checks that cleanup does not remove
// workloads belonging to other units in the shared containerd namespace.
func (s *runtimeSuite) TestRemoveUnitOnlyTouchesOwnedTasks(c *tc.C) {
	f := newRuntimeFixture(c)
	owned := NewMockContainer(f.ctrl)
	other := NewMockContainer(f.ctrl)
	f.client.EXPECT().Containers(gomock.Any()).Return([]client.Container{owned, other}, nil)
	f.client.EXPECT().Close().Return(nil)
	owned.EXPECT().Labels(gomock.Any()).Return(map[string]string{unitLabel: f.spec.UnitName}, nil)
	owned.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	owned.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
	other.EXPECT().Labels(gomock.Any()).Return(map[string]string{unitLabel: "other/0"}, nil)

	c.Assert(f.runtime.RemoveUnit(c.Context(), f.spec.UnitName), tc.ErrorIsNil)
}

// TestMissingImageDoesNotCreateContainer checks that an image missing from
// the content store is reported instead of starting a stale image revision.
func (s *runtimeSuite) TestMissingImageDoesNotCreateContainer(c *tc.C) {
	f := newRuntimeFixture(c)
	f.client.EXPECT().GetImage(gomock.Any(), f.spec.Image).Return(nil, errdefs.ErrNotFound)
	f.client.EXPECT().Close().Return(nil)

	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `looking up pulled OCI image .*not found.*`)
}

// TestTaskStopAfterGraceTimeout verifies that a task gets a fresh bounded
// context for forced deletion after its graceful termination times out.
func (s *runtimeSuite) TestTaskStopAfterGraceTimeout(c *tc.C) {
	ctrl := gomock.NewController(c)
	task := NewMockTask(ctrl)
	task.EXPECT().Status(gomock.Any()).Return(client.Status{Status: client.Running}, nil)
	task.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) (<-chan client.ExitStatus, error) {
		return make(chan client.ExitStatus), nil
	})
	task.EXPECT().Kill(gomock.Any(), syscall.SIGTERM).Return(nil)
	task.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ ...client.ProcessDeleteOpts) (*client.ExitStatus, error) {
		if ctx.Err() != nil {
			return nil, errors.New("delete context expired")
		}
		return nil, nil
	})
	_ = c.Context()

	synctest.Test(c.T, func(_ *testing.T) {
		if err := stopTask(context.Background(), task); err != nil {
			panic(err)
		}
	})
}

// TestSpecFingerprintChanges verifies that image revisions and user metadata
// cause task replacement while a stable spec allows adoption.
func (s *runtimeSuite) TestSpecFingerprintChanges(c *tc.C) {
	f := newRuntimeFixture(c)
	first, err := workloadLabels(f.spec, digest.FromString("image-1").String())
	c.Assert(err, tc.ErrorIsNil)
	second, err := workloadLabels(f.spec, digest.FromString("image-1").String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(first, tc.DeepEquals, second)

	uid := 10001
	f.spec.UID = &uid
	changed, err := workloadLabels(f.spec, digest.FromString("image-1").String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(first[specLabel] == changed[specLabel], tc.IsFalse)

	otherImage, err := workloadLabels(f.spec, digest.FromString("image-2").String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(changed[imageLabel] == otherImage[imageLabel], tc.IsFalse)
	c.Check(strings.HasPrefix(workloadID(f.spec.UnitName, f.spec.Name), "juju-"), tc.IsTrue)
}
