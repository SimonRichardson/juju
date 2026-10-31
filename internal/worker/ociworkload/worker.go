// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/juju/clock"
	"github.com/juju/worker/v5"
	"gopkg.in/tomb.v2"
	"gopkg.in/yaml.v3"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/core/watcher"
	charmmeta "github.com/juju/juju/domain/deployment/charm"
	charmresource "github.com/juju/juju/domain/deployment/charm/resource"
	"github.com/juju/juju/internal/docker"
	"github.com/juju/juju/internal/worker/fortress"
)

const (
	maxImageResourceSize = 1 << 20
	retryInterval        = 5 * time.Second
	refreshInterval      = 5 * time.Minute
)

var errCharmNotReady = errors.New("charm directory not ready")

// Application supplies notifications and the current charm URL for the
// application of an assigned unit. Watch must send an initial event.
type Application interface {
	Watch(context.Context) (watcher.NotifyWatcher, error)
	CharmURL(context.Context) (string, bool, error)
}

// ResourceReader obtains OCI image resources for the assigned unit. A
// successful call must return a non-nil stream, which the caller closes.
type ResourceReader interface {
	GetResource(context.Context, string) (resource.Resource, io.ReadCloser, error)
}

// ImagePuller downloads and unpacks an OCI image into the machine runtime.
// Pull must honour cancellation and be safe to retry for the same image.
type ImagePuller interface {
	Pull(context.Context, docker.DockerImageDetails) error
}

// WorkerConfig contains dependencies of one unit's image reconciler.
type WorkerConfig struct {
	// Application supplies the subscribed charm state for the unit.
	Application Application
	// Resources provides the assigned unit's image resource contents.
	Resources ResourceReader
	// CharmDir is the directory into which the uniter deploys this charm.
	CharmDir string
	// CharmDirGuest guards reads while the uniter modifies CharmDir.
	CharmDirGuest fortress.Guest
	// Puller performs downloads using the host container runtime.
	Puller ImagePuller
	// Clock schedules retries and periodic resource reconciliation.
	Clock clock.Clock
	// Logger reports failures without logging image credentials.
	Logger logger.Logger
}

func (c WorkerConfig) validate() error {
	switch {
	case c.Application == nil:
		return errors.New("missing Application")
	case c.Resources == nil:
		return errors.New("missing Resources")
	case c.CharmDir == "":
		return errors.New("missing CharmDir")
	case c.CharmDirGuest == nil:
		return errors.New("missing CharmDirGuest")
	case c.Puller == nil:
		return errors.New("missing Puller")
	case c.Clock == nil:
		return errors.New("missing Clock")
	case c.Logger == nil:
		return errors.New("missing Logger")
	}
	return nil
}

type workloadWorker struct {
	config WorkerConfig
	tomb   tomb.Tomb

	// Successful pulls are remembered for the lifetime of this worker.
	pulled map[string][sha256.Size]byte
}

// NewWorker starts a reconciler for one already-assigned machine unit. A
// restart repeats image pulls safely; Kill cancels in-flight API calls and
// downloads, and Wait returns only after the watcher has been stopped.
func NewWorker(config WorkerConfig) (worker.Worker, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	w := &workloadWorker{config: config, pulled: make(map[string][sha256.Size]byte)}
	w.tomb.Go(w.run)

	return w, nil
}

// Kill requests a cancellation-aware stop of the reconciler.
func (w *workloadWorker) Kill() {
	w.tomb.Kill(nil)
}

// Wait blocks until the reconciler and its application watcher have stopped.
func (w *workloadWorker) Wait() error {
	return w.tomb.Wait()
}

func (w *workloadWorker) run() (err error) {
	ctx := w.tomb.Context(context.Background())

	// Establish the subscription before querying the charm or the resources.
	watch, err := w.config.Application.Watch(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if stopErr := worker.Stop(watch); err == nil {
			err = stopErr
		}
	}()
	// A failed watcher need not close its Changes channel. Monitor its
	// lifecycle so a dead subscription restarts this worker.
	w.tomb.Go(func() error {
		watchErr := watch.Wait()
		select {
		case <-w.tomb.Dying():
			return nil
		default:
			if watchErr == nil {
				return errors.New("application watcher stopped")
			}
			return watchErr
		}
	})

	// The first notification is the watcher's readiness barrier, even if it
	// contains no changes. Never discard it before querying current state.
	select {
	case _, ok := <-watch.Changes():
		if !ok {
			return errors.New("application watcher closed before initial event")
		}
	case <-ctx.Done():
		return nil
	}

	interval := w.reconcile(ctx)
	timer := w.config.Clock.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case _, ok := <-watch.Changes():
			if !ok {
				return errors.New("application watcher closed")
			}
		case <-timer.Chan():
		case <-ctx.Done():
			return nil
		}

		interval = w.reconcile(ctx)
		timer.Reset(interval)
	}
}

// reconcile returns the interval until the next attempt. Charm URLs and OCI
// resources may change independently, so re-query after both notifications
// and a periodic refresh.
func (w *workloadWorker) reconcile(ctx context.Context) time.Duration {
	if err := w.pullImages(ctx); err != nil {
		if !errors.Is(err, errCharmNotReady) && ctx.Err() == nil {
			w.config.Logger.Errorf(ctx, "reconciling OCI images: %v", err)
		}
		return retryInterval
	}

	return refreshInterval
}

func (w *workloadWorker) pullImages(ctx context.Context) error {
	url, _, err := w.config.Application.CharmURL(ctx)
	if err != nil {
		return err
	}
	if url == "" {
		return errCharmNotReady
	}

	// The uniter locks this fortress while replacing the charm directory.
	// Read only inside Visit so we never observe partially written metadata.
	var meta *charmmeta.Meta
	err = w.config.CharmDirGuest.Visit(ctx, func() error {
		data, err := os.ReadFile(filepath.Join(w.config.CharmDir, ".juju-charm"))
		if os.IsNotExist(err) {
			return errCharmNotReady
		} else if err != nil {
			return err
		}

		var deployedURL string
		if err := yaml.Unmarshal(data, &deployedURL); err != nil {
			return err
		}
		if deployedURL != url {
			return errCharmNotReady
		}

		meta, err = charmmeta.ReadCharmDirMetadata(w.config.CharmDir)
		return err
	})
	if err != nil {
		return err
	}

	containerNames := make([]string, 0, len(meta.Containers))
	for name := range meta.Containers {
		containerNames = append(containerNames, name)
	}
	slices.Sort(containerNames)

	for _, name := range containerNames {
		resourceName := meta.Containers[name].Resource
		if resourceName == "" {
			return fmt.Errorf("container %q has no OCI image resource", name)
		}

		image, fingerprint, err := w.imageResource(ctx, resourceName)
		if err != nil {
			return fmt.Errorf("loading image resource %q: %w", resourceName, err)
		}
		if w.pulled[resourceName] == fingerprint {
			continue
		}

		if err := w.config.Puller.Pull(ctx, image); err != nil {
			return fmt.Errorf("pulling image for container %q: %w", name, err)
		}
		w.pulled[resourceName] = fingerprint
	}

	return nil
}

func (w *workloadWorker) imageResource(ctx context.Context, name string) (docker.DockerImageDetails, [sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	info, reader, err := w.config.Resources.GetResource(ctx, name)
	if err != nil {
		return docker.DockerImageDetails{}, zero, err
	}
	if reader == nil {
		return docker.DockerImageDetails{}, zero, errors.New("OCI image resource has no content")
	}

	if info.Type != charmresource.TypeContainerImage {
		_ = reader.Close()
		return docker.DockerImageDetails{}, zero, errors.New("resource is not an OCI image")
	}
	// Bound memory use even if the API returns an incorrect resource size.
	data, readErr := io.ReadAll(io.LimitReader(reader, maxImageResourceSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return docker.DockerImageDetails{}, zero, readErr
	}
	if closeErr != nil {
		return docker.DockerImageDetails{}, zero, closeErr
	}
	if len(data) > maxImageResourceSize {
		return docker.DockerImageDetails{}, zero, errors.New("OCI image resource exceeds maximum size")
	}

	image, err := docker.UnmarshalDockerResource(data)
	if err != nil {
		return docker.DockerImageDetails{}, zero, err
	}
	if err := docker.ValidateDockerRegistryPath(image.RegistryPath); err != nil {
		return docker.DockerImageDetails{}, zero, err
	}

	return image, sha256.Sum256(data), nil
}
