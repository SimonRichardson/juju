// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/canonical/gomock/gomock"
	"github.com/juju/clock"
	"github.com/juju/tc"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/workertest"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/core/watcher/watchertest"
	charmresource "github.com/juju/juju/domain/deployment/charm/resource"
	"github.com/juju/juju/internal/docker"
	internallogger "github.com/juju/juju/internal/logger"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/ociworkload"
)

const imageMetadata = `name: redis-k8s
summary: Redis workload
description: Redis workload
containers:
  redis:
    resource: redis-image
resources:
  redis-image:
    type: oci-image
    description: Redis image
`

type workerSuite struct {
	coretesting.BaseSuite
}

// TestWorkerSuite exercises OCI image reconciliation for assigned units.
func TestWorkerSuite(t *testing.T) {
	tc.Run(t, &workerSuite{})
}

type workerFixture struct {
	application   *MockApplication
	resources     *MockResourceReader
	puller        *MockImagePuller
	runtime       *MockContainerRuntime
	changes       chan struct{}
	watch         *watchertest.MockNotifyWatcher
	queries       chan struct{}
	clock         clock.Clock
	charmDir      string
	worker        worker.Worker
	customRuntime bool
	logger        logger.Logger
}

func newWorkerFixture(c *tc.C, metadata string) *workerFixture {
	ctrl := gomock.NewController(c)
	fixture := &workerFixture{
		application: NewMockApplication(ctrl),
		resources:   NewMockResourceReader(ctrl),
		puller:      NewMockImagePuller(ctrl),
		runtime:     NewMockContainerRuntime(ctrl),
		clock:       clock.WallClock,
		charmDir:    c.MkDir(),
		logger:      loggertesting.WrapCheckLog(c),
	}
	c.Assert(os.WriteFile(filepath.Join(fixture.charmDir, ".juju-charm"), []byte("ch:redis-k8s-1\n"), 0600), tc.ErrorIsNil)
	c.Assert(os.WriteFile(filepath.Join(fixture.charmDir, "metadata.yaml"), []byte(metadata), 0600), tc.ErrorIsNil)

	return fixture
}

func (f *workerFixture) start(c *tc.C) {
	c.Assert(f.startWorker(), tc.ErrorIsNil)
}

func (f *workerFixture) startWorker() error {
	f.changes = make(chan struct{}, 1)
	f.queries = make(chan struct{}, 4)
	f.watch = watchertest.NewMockNotifyWatcher(f.changes)
	f.application.EXPECT().Watch(gomock.Any()).Return(f.watch, nil)
	f.application.EXPECT().CharmURL(gomock.Any()).DoAndReturn(func(context.Context) (string, bool, error) {
		f.queries <- struct{}{}
		return "ch:redis-k8s-1", false, nil
	}).AnyTimes()

	if !f.customRuntime {
		f.runtime.EXPECT().Ensure(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		f.runtime.EXPECT().RemoveUnit(gomock.Any(), "redis/0").Return(nil).AnyTimes()
	}

	w, err := ociworkload.NewWorker(ociworkload.WorkerConfig{
		Application:   f.application,
		Resources:     f.resources,
		CharmDir:      f.charmDir,
		Puller:        f.puller,
		Runtime:       f.runtime,
		UnitName:      "redis/0",
		PebbleBinary:  filepath.Join(f.charmDir, "pebble"),
		ContainersDir: filepath.Join(f.charmDir, "containers"),
		StateDir:      filepath.Join(f.charmDir, "oci"),
		Clock:         f.clock,
		Logger:        f.logger,
	})
	if err != nil {
		return err
	}
	f.worker = w
	return nil
}

func (f *workerFixture) stop(c *tc.C) {
	workertest.CleanKill(c, f.worker)
}

func imageResource(body string, resourceType charmresource.Type) (resource.Resource, io.ReadCloser, error) {
	return resource.Resource{Resource: charmresource.Resource{
		Meta: charmresource.Meta{Type: resourceType},
	}}, io.NopCloser(strings.NewReader(body)), nil
}

// TestPullAfterInitialWatchEvent ensures subscription readiness precedes
// querying the charm and downloading its image resource.
func (s *workerSuite) TestPullAfterInitialWatchEvent(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource(`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage))
	pulled := make(chan docker.DockerImageDetails, 1)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, image docker.DockerImageDetails) error {
		pulled <- image
		return nil
	})

	f.start(c)
	defer f.stop(c)
	select {
	case <-f.queries:
		c.Fatal("queried charm before the watcher's initial event")
	default:
	}
	f.changes <- struct{}{}

	select {
	case image := <-pulled:
		c.Check(image.RegistryPath, tc.Equals, "docker.io/library/redis:7")
	case <-c.Context().Done():
		c.Fatal("image was not pulled")
	}
}

// TestRefreshPullsChangedResource checks JSON and YAML image resources and
// ensures an application change causes a new image pull.
func (s *workerSuite) TestRefreshPullsChangedResource(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource(`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage))
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource("registrypath: docker.io/library/redis:8", charmresource.TypeContainerImage))
	pulled := make(chan string, 2)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, image docker.DockerImageDetails) error {
		pulled <- image.RegistryPath
		return nil
	}).Times(2)

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	select {
	case image := <-pulled:
		c.Check(image, tc.Equals, "docker.io/library/redis:7")
	case <-c.Context().Done():
		c.Fatal("first image was not pulled")
	}
	f.changes <- struct{}{}
	select {
	case image := <-pulled:
		c.Check(image, tc.Equals, "docker.io/library/redis:8")
	case <-c.Context().Done():
		c.Fatal("refreshed image was not pulled")
	}
}

// TestPrivateImageDetailsReachPuller checks that registry credentials from a
// unit resource reach the runtime without being read from charm metadata.
func (s *workerSuite) TestPrivateImageDetailsReachPuller(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource(
		`{"ImageName":"registry.example.com/redis:1","username":"user","password":"secret"}`,
		charmresource.TypeContainerImage,
	))
	pulled := make(chan docker.DockerImageDetails, 1)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, image docker.DockerImageDetails) error {
		pulled <- image
		return nil
	})

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	select {
	case image := <-pulled:
		c.Check(image.RegistryPath, tc.Equals, "registry.example.com/redis:1")
		c.Check(image.Username, tc.Equals, "user")
		c.Check(image.Password, tc.Equals, "secret")
	case <-c.Context().Done():
		c.Fatal("private image was not passed to the runtime")
	}
}

// TestInvalidResourceTypeRecoversOnChange checks that a non-image resource is
// rejected and a subsequent corrected resource is retried on notification.
func (s *workerSuite) TestInvalidResourceTypeRecoversOnChange(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	read := make(chan struct{}, 1)
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").DoAndReturn(func(context.Context, string) (resource.Resource, io.ReadCloser, error) {
		read <- struct{}{}
		return imageResource("not an image", charmresource.TypeFile)
	})
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource(
		`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage,
	))
	pulled := make(chan struct{}, 1)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, docker.DockerImageDetails) error {
		pulled <- struct{}{}
		return nil
	})

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	select {
	case <-read:
	case <-c.Context().Done():
		c.Fatal("resource was not checked")
	}
	f.changes <- struct{}{}
	select {
	case <-pulled:
	case <-c.Context().Done():
		c.Fatal("corrected image was not pulled")
	}
}

// TestPullFailureRetries uses synctest fake time to exercise the retry after a
// failed download without waiting for wall-clock time.
func (s *workerSuite) TestPullFailureRetries(c *tc.C) {
	// tc owns the temporary files and mocks outside the fake-time bubble.
	f := newWorkerFixture(c, imageMetadata)
	f.logger = internallogger.Noop()
	_ = c.Context()

	synctest.Test(c.T, func(_ *testing.T) {
		f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").DoAndReturn(func(context.Context, string) (resource.Resource, io.ReadCloser, error) {
			return imageResource(`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage)
		}).Times(2)
		f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).Return(errors.New("registry unavailable"))
		retried := make(chan struct{}, 1)
		f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, docker.DockerImageDetails) error {
			retried <- struct{}{}
			return nil
		})

		if err := f.startWorker(); err != nil {
			panic(err)
		}
		defer func() {
			f.worker.Kill()
			if err := f.worker.Wait(); err != nil {
				panic(err)
			}
		}()
		f.changes <- struct{}{}
		synctest.Wait()
		synctest.Sleep(5 * time.Second)
		select {
		case <-retried:
		default:
			c.Fatal("failed image pull was not retried")
		}
	})
}

// TestOrdinaryMachineCharmDoesNotFetchImages ensures unrelated units do not
// request OCI resources or connect to containerd.
func (s *workerSuite) TestOrdinaryMachineCharmDoesNotFetchImages(c *tc.C) {
	f := newWorkerFixture(c, "name: redis\nsummary: Redis\ndescription: Redis\n")

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	// The query proves the reconcile read past the charm URL; any
	// attempted resource fetch or containerd call fails the strict mocks.
	select {
	case <-f.queries:
	case <-c.Context().Done():
		c.Fatal("did not inspect the assigned charm")
	}
}

// TestCancellationBeforeInitialEvent checks shutdown while subscription
// readiness is still outstanding.
func (s *workerSuite) TestCancellationBeforeInitialEvent(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	f.start(c)
	f.stop(c)
}

// TestReconcilesOnceCharmDirAppears checks the worker does not block or die
// while the uniter is still deploying the charm. The worker must retry
// silently and converge the workloads as soon as the charm directory is
// complete, so containers exist before the install hook runs.
func (s *workerSuite) TestReconcilesOnceCharmDirAppears(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	// A unit agent starts before the uniter has deployed any charm.
	c.Assert(os.RemoveAll(f.charmDir), tc.ErrorIsNil)

	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").DoAndReturn(func(context.Context, string) (resource.Resource, io.ReadCloser, error) {
		return imageResource(`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage)
	}).AnyTimes()
	pulled := make(chan struct{}, 1)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, docker.DockerImageDetails) error {
		pulled <- struct{}{}
		return nil
	})

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	select {
	case <-pulled:
		c.Fatal("pulled an image before the charm was deployed")
	default:
	}

	// The uniter finishes deploying the charm, stamping it complete.
	c.Assert(os.Mkdir(f.charmDir, 0755), tc.ErrorIsNil)
	c.Assert(os.WriteFile(filepath.Join(f.charmDir, ".juju-charm"), []byte("ch:redis-k8s-1\n"), 0600), tc.ErrorIsNil)
	c.Assert(os.WriteFile(filepath.Join(f.charmDir, "metadata.yaml"), []byte(imageMetadata), 0600), tc.ErrorIsNil)

	f.changes <- struct{}{}
	select {
	case <-pulled:
	case <-c.Context().Done():
		c.Fatal("did not reconcile once the charm appeared")
	}
}

// TestWatcherFailureStopsWorker checks that a watcher can terminate the worker
// even if its change stream remains open after failure.
func (s *workerSuite) TestWatcherFailureStopsWorker(c *tc.C) {
	f := newWorkerFixture(c, imageMetadata)
	f.start(c)
	f.watch.KillErr(errors.New("watch failed"))
	c.Check(f.worker.Wait(), tc.ErrorMatches, "watch failed")
}

const imageMetadataTwoContainers = `name: redis-k8s
summary: Redis workload
description: Redis workload
containers:
  redis:
    resource: redis-image
  mysql:
    resource: mysql-image
resources:
  redis-image:
    type: oci-image
    description: Redis image
  mysql-image:
    type: oci-image
    description: MySQL image
`

// TestRetiresContainersRemovedFromMetadata checks a charm upgrade dropping
// one of several containers retires exactly the removed workload, like
// Kubernetes recreating a pod with only the declared containers.
func (s *workerSuite) TestRetiresContainersRemovedFromMetadata(c *tc.C) {
	f := newWorkerFixture(c, imageMetadataTwoContainers)
	// The kept container is re-read on every reconcile; the removed one
	// only before the upgrade.
	f.resources.EXPECT().GetResource(gomock.Any(), "redis-image").Return(imageResource(
		`{"ImageName":"docker.io/library/redis:7"}`, charmresource.TypeContainerImage)).Times(2)
	f.resources.EXPECT().GetResource(gomock.Any(), "mysql-image").Return(imageResource(
		`{"ImageName":"docker.io/library/mysql:8"}`, charmresource.TypeContainerImage))
	pulled := make(chan string, 2)
	f.puller.EXPECT().Pull(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, image docker.DockerImageDetails) error {
		pulled <- image.RegistryPath
		return nil
	}).Times(2)
	// The upgraded metadata keeps redis, so mysql is retired.
	retired := make(chan []string, 1)
	f.runtime.EXPECT().RemoveContainers(gomock.Any(), "redis/0", []string{"redis"}).DoAndReturn(
		func(_ context.Context, _ string, keep []string) error {
			retired <- keep
			return nil
		})

	f.start(c)
	defer f.stop(c)
	f.changes <- struct{}{}
	for range 2 {
		select {
		case <-pulled:
		case <-c.Context().Done():
			c.Fatal("initial images were not pulled")
		}
	}

	// Upgrade the charm to metadata declaring only redis.
	c.Assert(os.WriteFile(filepath.Join(f.charmDir, "metadata.yaml"), []byte(imageMetadata), 0644), tc.ErrorIsNil)
	f.changes <- struct{}{}
	select {
	case keep := <-retired:
		c.Check(keep, tc.DeepEquals, []string{"redis"})
	case <-c.Context().Done():
		c.Fatal("removed container was not retired")
	}
}
