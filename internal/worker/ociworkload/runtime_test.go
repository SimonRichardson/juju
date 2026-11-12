// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/canonical/gomock/gomock"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/juju/tc"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

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
	spec           ContainerSpec
	runtime        ContainerdRuntime
	client         *MockContainerdClient
	image          *MockImage
	ctrl           *gomock.Controller
	charmLayoutDir string
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
			UnitName:     "redis/0",
			Name:         "redis",
			Image:        "docker.io/library/redis:7",
			PebbleBinary: pebble,
			// The shared socket directory follows the unit agent layout,
			// which the mirrored socket ownership check validates.
			ContainersDir: filepath.Join(dir, "agents", "unit-redis-0", "containers"),
			StateDir:      filepath.Join(dir, "oci"),
		},
		charmLayoutDir: filepath.Join(dir, "charm"),
	}
	fixture.runtime = ContainerdRuntime{
		NewClient: func() (ContainerdClient, error) {
			return daemon, nil
		},
		CharmLayoutDir: fixture.charmLayoutDir,
	}
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

// TestEnsureCreatesNewContainer checks the initial containerd provisioning
// path after an assigned unit's OCI resource has been downloaded.
func (s *runtimeSuite) TestEnsureCreatesNewContainer(c *tc.C) {
	f := newRuntimeFixture(c)
	f.expectImage()
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(nil, errdefs.ErrNotFound)
	container := NewMockContainer(f.ctrl)
	task := NewMockTask(f.ctrl)
	f.client.EXPECT().NewContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(container, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	container.EXPECT().NewTask(gomock.Any(), gomock.Any()).Return(task, nil)
	task.EXPECT().Start(gomock.Any()).Return(nil)

	c.Assert(f.runtime.Ensure(c.Context(), f.spec), tc.ErrorIsNil)
}

// TestEnsureRefreshReplacesTask checks image digest changes stop the old
// Pebble task before starting the new OCI image for that same unit.
func (s *runtimeSuite) TestEnsureRefreshReplacesTask(c *tc.C) {
	f := newRuntimeFixture(c)
	f.expectImage()
	oldContainer := NewMockContainer(f.ctrl)
	oldTask := NewMockTask(f.ctrl)
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(oldContainer, nil)
	oldContainer.EXPECT().Labels(gomock.Any()).Return(map[string]string{
		unitLabel:      f.spec.UnitName,
		containerLabel: f.spec.Name,
		imageLabel:     "old digest",
	}, nil)
	oldContainer.EXPECT().Task(gomock.Any(), nil).Return(oldTask, nil)
	oldTask.EXPECT().Status(gomock.Any()).Return(client.Status{Status: client.Running}, nil)
	stop := make(chan client.ExitStatus, 1)
	stop <- *client.NewExitStatus(0, time.Time{}, nil)
	oldTask.EXPECT().Wait(gomock.Any()).Return(stop, nil)
	oldTask.EXPECT().Kill(gomock.Any(), syscall.SIGTERM).Return(nil)
	oldTask.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil, nil)
	oldContainer.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)

	newContainer := NewMockContainer(f.ctrl)
	newTask := NewMockTask(f.ctrl)
	f.client.EXPECT().NewContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(newContainer, nil)
	newContainer.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	newContainer.EXPECT().NewTask(gomock.Any(), gomock.Any()).Return(newTask, nil)
	newTask.EXPECT().Start(gomock.Any()).Return(nil)

	c.Assert(f.runtime.Ensure(c.Context(), f.spec), tc.ErrorIsNil)
}

// TestEnsureTaskStartFailureCleansUp ensures a failed Pebble process does
// not leave a task or snapshot that prevents the worker from retrying.
func (s *runtimeSuite) TestEnsureTaskStartFailureCleansUp(c *tc.C) {
	f := newRuntimeFixture(c)
	f.expectImage()
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(nil, errdefs.ErrNotFound)
	container := NewMockContainer(f.ctrl)
	task := NewMockTask(f.ctrl)
	f.client.EXPECT().NewContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(container, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	container.EXPECT().NewTask(gomock.Any(), gomock.Any()).Return(task, nil)
	task.EXPECT().Start(gomock.Any()).Return(errors.New("cannot start Pebble"))
	task.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil, nil)
	container.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)

	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `starting Pebble task: cannot start Pebble`)
}

// TestEnsureConnectionFailure reports an unavailable containerd daemon
// without attempting to create a workload task.
func (s *runtimeSuite) TestEnsureConnectionFailure(c *tc.C) {
	f := newRuntimeFixture(c)
	f.runtime.NewClient = func() (ContainerdClient, error) {
		return nil, errors.New("containerd unavailable")
	}
	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `connecting to containerd: containerd unavailable`)
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

// TestPebbleSpecMatchesKubernetesWorkload checks that OCI image entrypoints
// cannot replace Pebble, sockets and state are bind-mounted, and network
// identity matches the assigned machine rather than a private unconfigured NS.
func (s *runtimeSuite) TestPebbleSpecMatchesKubernetesWorkload(c *tc.C) {
	f := newRuntimeFixture(c)
	uid, gid := 10001, 10002
	f.spec.UID, f.spec.GID = &uid, &gid

	options := append([]oci.SpecOpts{oci.WithDefaultSpec()}, pebbleSpecOptions(f.spec)...)
	got, err := oci.GenerateSpec(namespaces.WithNamespace(c.Context(), namespace), nil, &containers.Container{ID: "redis"}, options...)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(got.Process, tc.NotNil)
	c.Check(got.Process.Args, tc.DeepEquals, []string{
		"/charm/bin/pebble", "run", "--create-dirs", "--hold", "--verbose",
	})
	c.Check(got.Process.User.UID, tc.Equals, uint32(uid))
	c.Check(got.Process.User.GID, tc.Equals, uint32(gid))
	for _, env := range []string{
		"JUJU_CONTAINER_NAME=redis",
		"PEBBLE_SOCKET=/charm/container/pebble.socket",
		"PEBBLE_COPY_ONCE=/var/lib/pebble/default",
		// Nothing may be mounted over the image's baked-in Pebble layers:
		// PEBBLE_COPY_ONCE seeds them from /var/lib/pebble/default into
		// $PEBBLE, which points into the mounted socket directory.
	} {
		c.Check(slices.Contains(got.Process.Env, env), tc.IsTrue)
	}
	for _, mount := range []struct {
		path   string
		source string
	}{
		{path: "/charm/bin/pebble", source: f.spec.PebbleBinary},
		{path: "/charm/container", source: filepath.Join(f.spec.ContainersDir, f.spec.Name)},
	} {
		c.Check(slices.ContainsFunc(got.Mounts, func(m specs.Mount) bool {
			return m.Destination == mount.path && m.Source == mount.source
		}), tc.IsTrue)
	}
	// The image's baked-in Pebble layers must stay visible for
	// PEBBLE_COPY_ONCE, so nothing may be mounted over /var/lib/pebble.
	c.Check(slices.ContainsFunc(got.Mounts, func(m specs.Mount) bool {
		return m.Destination == "/var/lib/pebble/default"
	}), tc.IsFalse)
	c.Assert(got.Linux, tc.NotNil)
	c.Check(slices.ContainsFunc(got.Linux.Namespaces, func(ns specs.LinuxNamespace) bool {
		return ns.Type == specs.NetworkNamespace
	}), tc.IsFalse)
}

// TestEnsureMirrorsCharmContainerLayout checks the runtime presents the
// paths a Kubernetes charm container would, so unmodified sidecar charms
// resolve their workload sockets and the Pebble binary.
func (s *runtimeSuite) TestEnsureMirrorsCharmContainerLayout(c *tc.C) {
	f := newRuntimeFixture(c)
	f.expectImage()
	container := NewMockContainer(f.ctrl)
	f.client.EXPECT().LoadContainer(gomock.Any(), gomock.Any()).Return(nil, errdefs.ErrNotFound)
	f.client.EXPECT().NewContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(container, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	task := NewMockTask(f.ctrl)
	container.EXPECT().NewTask(gomock.Any(), gomock.Any()).Return(task, nil)
	task.EXPECT().Start(gomock.Any()).Return(nil)

	c.Assert(f.runtime.Ensure(c.Context(), f.spec), tc.ErrorIsNil)

	socketLink := filepath.Join(f.charmLayoutDir, "containers", f.spec.Name, "pebble.socket")
	target, err := os.Readlink(socketLink)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(target, tc.Equals, filepath.Join(f.spec.ContainersDir, f.spec.Name, "pebble.socket"))

	pebbleLink := filepath.Join(f.charmLayoutDir, "bin", "pebble")
	target, err = os.Readlink(pebbleLink)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(target, tc.Equals, f.spec.PebbleBinary)

	// Reconciling the mirrors again leaves them stable.
	c.Assert(f.runtime.mirrorCharmContainerLayout(f.spec), tc.ErrorIsNil)
	target, err = os.Readlink(socketLink)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(target, tc.Equals, filepath.Join(f.spec.ContainersDir, f.spec.Name, "pebble.socket"))
}

// TestEnsureRefusesForeignMirror checks a mirrored socket pointing at
// another unit's workload is never repointed, so same-named containers of
// different units cannot silently swap workloads. The mirror is created
// before the containerd connection, so no containerd calls are expected.
func (s *runtimeSuite) TestEnsureRefusesForeignMirror(c *tc.C) {
	f := newRuntimeFixture(c)

	link := filepath.Join(f.charmLayoutDir, "containers", f.spec.Name, "pebble.socket")
	c.Assert(os.MkdirAll(filepath.Dir(link), 0700), tc.ErrorIsNil)
	c.Assert(os.Symlink("/var/lib/juju/agents/unit-other-0/containers/redis/pebble.socket", link), tc.ErrorIsNil)

	err := f.runtime.Ensure(c.Context(), f.spec)
	c.Check(err, tc.ErrorMatches, `mirroring Pebble socket for container "redis": .*already points at.*`)
}

// TestRemoveContainersKeepsDeclaredContainers checks the container set is
// reconciled against the charm metadata: kept containers and their mirrors
// survive, undeclared ones are stopped and unmirrored.
func (s *runtimeSuite) TestRemoveContainersKeepsDeclaredContainers(c *tc.C) {
	f := newRuntimeFixture(c)
	socketLink := filepath.Join(f.charmLayoutDir, "containers", f.spec.Name, "pebble.socket")
	c.Assert(os.MkdirAll(filepath.Dir(socketLink), 0700), tc.ErrorIsNil)
	c.Assert(os.Symlink(
		filepath.Join(f.spec.ContainersDir, f.spec.Name, "pebble.socket"), socketLink), tc.ErrorIsNil)

	stale := NewMockContainer(f.ctrl)
	staleTask := NewMockTask(f.ctrl)
	kept := NewMockContainer(f.ctrl)
	f.client.EXPECT().Containers(gomock.Any()).Return([]client.Container{stale, kept}, nil)
	f.client.EXPECT().Close().Return(nil)
	stale.EXPECT().Labels(gomock.Any()).Return(map[string]string{
		unitLabel:      f.spec.UnitName,
		containerLabel: "mysql",
	}, nil)
	stale.EXPECT().Task(gomock.Any(), nil).Return(staleTask, nil)
	staleTask.EXPECT().Status(gomock.Any()).Return(client.Status{Status: client.Running}, nil)
	stop := make(chan client.ExitStatus, 1)
	stop <- *client.NewExitStatus(0, time.Time{}, nil)
	staleTask.EXPECT().Wait(gomock.Any()).Return(stop, nil)
	staleTask.EXPECT().Kill(gomock.Any(), syscall.SIGTERM).Return(nil)
	staleTask.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil, nil)
	stale.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
	kept.EXPECT().Labels(gomock.Any()).Return(map[string]string{
		unitLabel:      f.spec.UnitName,
		containerLabel: f.spec.Name,
	}, nil)

	c.Assert(f.runtime.RemoveContainers(c.Context(), f.spec.UnitName, []string{f.spec.Name}), tc.ErrorIsNil)

	_, err := os.Lstat(socketLink)
	c.Assert(err, tc.ErrorIsNil)
}

// TestRemoveContainersRemovesMirror checks retiring a container also
// retires its mirrored socket, but leaves mirrors of other units alone.
func (s *runtimeSuite) TestRemoveContainersRemovesMirror(c *tc.C) {
	f := newRuntimeFixture(c)
	owned := filepath.Join(f.charmLayoutDir, "containers", f.spec.Name, "pebble.socket")
	c.Assert(os.MkdirAll(filepath.Dir(owned), 0700), tc.ErrorIsNil)
	c.Assert(os.Symlink(
		filepath.Join(f.spec.ContainersDir, f.spec.Name, "pebble.socket"), owned), tc.ErrorIsNil)
	foreign := filepath.Join(f.charmLayoutDir, "containers", "mysql", "pebble.socket")
	c.Assert(os.MkdirAll(filepath.Dir(foreign), 0700), tc.ErrorIsNil)
	// A container of another unit on the same machine; the link points
	// into that unit's agents directory and must be left alone.
	foreignTarget := "/var/lib/juju/agents/unit-other-0/containers/mysql/pebble.socket"
	c.Assert(os.Symlink(foreignTarget, foreign), tc.ErrorIsNil)

	container := NewMockContainer(f.ctrl)
	f.client.EXPECT().Containers(gomock.Any()).Return([]client.Container{container}, nil)
	f.client.EXPECT().Close().Return(nil)
	container.EXPECT().Labels(gomock.Any()).Return(map[string]string{
		unitLabel:      f.spec.UnitName,
		containerLabel: f.spec.Name,
	}, nil)
	container.EXPECT().Task(gomock.Any(), nil).Return(nil, errdefs.ErrNotFound)
	container.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)

	c.Assert(f.runtime.RemoveUnit(c.Context(), f.spec.UnitName), tc.ErrorIsNil)

	_, err := os.Lstat(owned)
	c.Check(os.IsNotExist(err), tc.IsTrue)

	target, err := os.Readlink(foreign)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(target, tc.Equals, foreignTarget)
}
