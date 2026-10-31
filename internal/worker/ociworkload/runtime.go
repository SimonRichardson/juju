// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	unitLabel      = "com.canonical.juju.unit"
	containerLabel = "com.canonical.juju.container"
	imageLabel     = "com.canonical.juju.image"
	specLabel      = "com.canonical.juju.spec"
	stopDelay      = 10 * time.Second
)

// ContainerSpec describes one Pebble-managed OCI workload on a Juju machine.
// The containerd task uses the machine's network namespace, matching the
// network identity advertised for machine units; no Pebble HTTP port is
// exposed, because each workload has its own Unix socket.
type ContainerSpec struct {
	// UnitName and Name identify the assigned unit and metadata container.
	UnitName string
	Name     string
	// Image is the registry reference already pulled into containerd.
	Image string
	// PebbleBinary is the agent-tools Pebble binary mounted read-only into the
	// image and launched as PID 1.
	PebbleBinary string
	// ContainersDir holds unit-scoped Unix sockets. StateDir holds persistent
	// Pebble state and task logs across image refreshes and worker restarts.
	ContainersDir string
	StateDir      string
	// UID and GID override the image's user when supplied by charm metadata.
	UID *int
	GID *int
}

// ContainerRuntime converges each declared workload to a running Pebble task.
// Ensure is idempotent across worker and machine restarts. RemoveUnit stops
// only containers owned by the named unit; it retains persistent Pebble state
// so temporary agent restarts do not lose the charm's service plan.
type ContainerRuntime interface {
	Ensure(context.Context, ContainerSpec) error
	RemoveUnit(context.Context, string) error
}

// ContainerdClient is the subset of containerd used to pull images and
// reconcile unit workload tasks. Callers own and close each connection.
type ContainerdClient interface {
	Pull(context.Context, string, ...client.RemoteOpt) (client.Image, error)
	GetImage(context.Context, string) (client.Image, error)
	LoadContainer(context.Context, string) (client.Container, error)
	NewContainer(context.Context, string, ...client.NewContainerOpts) (client.Container, error)
	Containers(context.Context, ...string) ([]client.Container, error)
	Close() error
}

// ContainerdClientFactory returns a connection to the Juju namespace of the
// machine's containerd daemon. Callers must close successful connections.
type ContainerdClientFactory func() (ContainerdClient, error)

// NewContainerdClient connects to the machine runtime's Juju namespace.
// Unit manifolds supply this constructor to image pullers and task runtimes.
func NewContainerdClient() (ContainerdClient, error) {
	return client.New(defaults.DefaultAddress, client.WithDefaultNamespace(namespace))
}

// ContainerdRuntime manages workload tasks in Juju's containerd namespace.
type ContainerdRuntime struct {
	// NewClient is supplied by the unit manifold and owns each connection.
	NewClient ContainerdClientFactory
}

// Ensure starts Pebble as PID 1, or adopts a healthy task from a previous
// worker. An image or process-spec change replaces the old task atomically at
// the unit level: old task removal completes before starting the new one.
func (r ContainerdRuntime) Ensure(ctx context.Context, spec ContainerSpec) error {
	if err := spec.validate(); err != nil {
		return err
	}
	if _, err := os.Stat(spec.PebbleBinary); err != nil {
		return fmt.Errorf("accessing Pebble binary: %w", err)
	}
	if err := ensureWorkloadDirs(spec); err != nil {
		return err
	}

	conn, err := r.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to containerd: %w", err)
	}
	defer conn.Close()

	image, err := conn.GetImage(ctx, spec.Image)
	if err != nil {
		return fmt.Errorf("looking up pulled OCI image %q: %w", spec.Image, err)
	}
	labels, err := workloadLabels(spec, image.Target().Digest.String())
	if err != nil {
		return err
	}

	id := workloadID(spec.UnitName, spec.Name)
	container, err := conn.LoadContainer(ctx, id)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("loading workload container: %w", err)
	}
	if err == nil {
		currentLabels, err := container.Labels(ctx)
		if err != nil {
			return fmt.Errorf("reading workload ownership: %w", err)
		}
		if currentLabels[unitLabel] != spec.UnitName || currentLabels[containerLabel] != spec.Name {
			return fmt.Errorf("workload container %q belongs to another unit", id)
		}

		if currentLabels[imageLabel] == labels[imageLabel] && currentLabels[specLabel] == labels[specLabel] {
			task, err := container.Task(ctx, nil)
			if err == nil {
				status, err := task.Status(ctx)
				if err != nil {
					return fmt.Errorf("checking Pebble task: %w", err)
				}
				if status.Status == client.Running {
					return nil
				}
			} else if !errdefs.IsNotFound(err) {
				return fmt.Errorf("loading Pebble task: %w", err)
			}
			if err := startPebbleTask(ctx, container, spec); err != nil {
				return err
			}
			return nil
		}

		if err := removeContainer(ctx, container); err != nil {
			return err
		}
	}

	return createWorkload(ctx, conn, id, spec, image, labels)
}

// RemoveUnit stops every task owned by the unit, including containers no
// longer present in the current charm metadata. Images remain in the shared
// content store for other units and for subsequent restarts.
func (r ContainerdRuntime) RemoveUnit(ctx context.Context, unitName string) error {
	if unitName == "" {
		return errors.New("missing unit name")
	}
	conn, err := r.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to containerd: %w", err)
	}
	defer conn.Close()

	containers, err := conn.Containers(ctx)
	if err != nil {
		return fmt.Errorf("listing Juju workload containers: %w", err)
	}
	for _, container := range containers {
		labels, err := container.Labels(ctx)
		if err != nil {
			return fmt.Errorf("reading workload ownership: %w", err)
		}
		if labels[unitLabel] != unitName {
			continue
		}
		if err := removeContainer(ctx, container); err != nil {
			return err
		}
	}
	return nil
}

func (s ContainerSpec) validate() error {
	if s.UnitName == "" || s.Image == "" || s.PebbleBinary == "" || s.ContainersDir == "" || s.StateDir == "" {
		return errors.New("incomplete OCI workload specification")
	}
	if s.Name == "" || filepath.Base(s.Name) != s.Name || s.Name == "." || s.Name == ".." {
		return fmt.Errorf("invalid OCI container name %q", s.Name)
	}
	for _, id := range []*int{s.UID, s.GID} {
		if id != nil && (*id < 0 || *id > int(^uint32(0)>>1)) {
			return errors.New("OCI container user or group ID out of range")
		}
	}
	for _, path := range []string{s.PebbleBinary, s.ContainersDir, s.StateDir} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("OCI workload path %q must be absolute", path)
		}
	}
	return nil
}

func workloadID(unitName, name string) string {
	digest := sha256.Sum256([]byte(unitName + "/" + name))
	return "juju-" + hex.EncodeToString(digest[:16])
}

func workloadLabels(spec ContainerSpec, imageDigest string) (map[string]string, error) {
	// Include mounts and effective process identity in the label so an agent
	// restart can recognise when a running task has a stale specification.
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("encoding OCI workload specification: %w", err)
	}
	digest := sha256.Sum256(data)
	return map[string]string{
		unitLabel: spec.UnitName,
		containerLabel: spec.Name,
		imageLabel: imageDigest,
		specLabel: hex.EncodeToString(digest[:]),
	}, nil
}

func ensureWorkloadDirs(spec ContainerSpec) error {
	// A non-root image must be able to write its bind mounts. Keep their
	// parents private to the machine agent so other host users cannot reach
	// those writable paths or the Pebble control socket.
	for _, parent := range []string{spec.ContainersDir, spec.StateDir} {
		if err := os.MkdirAll(parent, 0700); err != nil {
			return fmt.Errorf("creating workload parent %q: %w", parent, err)
		}
		if err := os.Chmod(parent, 0700); err != nil {
			return fmt.Errorf("protecting workload parent %q: %w", parent, err)
		}
	}

	for _, dir := range []string{
		filepath.Join(spec.ContainersDir, spec.Name),
		filepath.Join(spec.StateDir, spec.Name, "pebble"),
	} {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return fmt.Errorf("creating workload directory %q: %w", dir, err)
		}
		// MkdirAll observes the current umask. Mounted directories must be
		// writable to images with a non-root default user, as with K8s EmptyDir.
		if err := os.Chmod(dir, 0777); err != nil {
			return fmt.Errorf("setting workload permissions %q: %w", dir, err)
		}
	}
	return nil
}

func createWorkload(ctx context.Context, conn ContainerdClient, id string, spec ContainerSpec, image client.Image, labels map[string]string) error {
	options := append([]oci.SpecOpts{oci.WithImageConfig(image)}, pebbleSpecOptions(spec)...)

	container, err := conn.NewContainer(ctx, id,
		client.WithImage(image),
		client.WithNewSnapshot(id+"-rootfs", image),
		client.WithNewSpec(options...),
		client.WithContainerLabels(labels))
	if err != nil {
		return fmt.Errorf("creating workload container: %w", err)
	}
	if err := startPebbleTask(ctx, container, spec); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopDelay)
		defer cancel()
		_ = container.Delete(cleanupCtx, client.WithSnapshotCleanup)
		return err
	}
	return nil
}

// pebbleSpecOptions mirror the K8s workload's command, environment and
// shared mounts. The caller applies image configuration first, then these
// overrides, so the image entrypoint cannot replace Pebble as PID 1.
func pebbleSpecOptions(spec ContainerSpec) []oci.SpecOpts {
	mounts := []specs.Mount{
		{Destination: "/charm/bin/pebble", Type: "bind", Source: spec.PebbleBinary, Options: []string{"rbind", "ro"}},
		{Destination: "/charm/container", Type: "bind", Source: filepath.Join(spec.ContainersDir, spec.Name), Options: []string{"rbind", "rw"}},
		{Destination: "/var/lib/pebble/default", Type: "bind", Source: filepath.Join(spec.StateDir, spec.Name, "pebble"), Options: []string{"rbind", "rw"}},
	}
	options := []oci.SpecOpts{
		oci.WithProcessArgs("/charm/bin/pebble", "run", "--create-dirs", "--hold", "--verbose"),
		oci.WithEnv([]string{
			"JUJU_CONTAINER_NAME=" + spec.Name,
			"PEBBLE_SOCKET=/charm/container/pebble.socket",
			"PEBBLE=/charm/container/pebble",
			"PEBBLE_COPY_ONCE=/var/lib/pebble/default",
		}),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithMounts(mounts),
	}
	if spec.UID != nil || spec.GID != nil {
		options = append(options, func(_ context.Context, _ oci.Client, _ *containers.Container, runtimeSpec *oci.Spec) error {
			if spec.UID != nil {
				runtimeSpec.Process.User.UID = uint32(*spec.UID)
			}
			if spec.GID != nil {
				runtimeSpec.Process.User.GID = uint32(*spec.GID)
			}
			return nil
		})
	}
	return options
}

func startPebbleTask(ctx context.Context, container client.Container, spec ContainerSpec) error {
	// A stopped task must be deleted before it can be recreated for the same
	// container. Its persistent Pebble plan lives in the mounted state dir.
	oldTask, err := container.Task(ctx, nil)
	if err == nil {
		if err := stopTask(ctx, oldTask); err != nil {
			return err
		}
	} else if !errdefs.IsNotFound(err) {
		return fmt.Errorf("loading existing Pebble task: %w", err)
	}

	logFile := filepath.Join(spec.StateDir, spec.Name, "pebble.log")
	task, err := container.NewTask(ctx, cio.LogFile(logFile))
	if err != nil {
		return fmt.Errorf("creating Pebble task: %w", err)
	}
	if err := task.Start(ctx); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopDelay)
		defer cancel()
		_, _ = task.Delete(cleanupCtx, client.WithProcessKill)
		return fmt.Errorf("starting Pebble task: %w", err)
	}
	return nil
}

func removeContainer(ctx context.Context, container client.Container) error {
	task, err := container.Task(ctx, nil)
	if err == nil {
		if err := stopTask(ctx, task); err != nil {
			return err
		}
	} else if !errdefs.IsNotFound(err) {
		return fmt.Errorf("loading workload task: %w", err)
	}
	if err := container.Delete(ctx, client.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("removing workload container: %w", err)
	}
	return nil
}

func stopTask(ctx context.Context, task client.Task) error {
	graceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopDelay)
	status, err := task.Status(graceCtx)
	if err != nil && !errdefs.IsNotFound(err) {
		cancel()
		return fmt.Errorf("checking workload task: %w", err)
	}
	if err == nil && status.Status == client.Running {
		wait, err := task.Wait(graceCtx)
		if err != nil {
			cancel()
			return fmt.Errorf("waiting for workload task: %w", err)
		}
		if err := task.Kill(graceCtx, syscall.SIGTERM); err != nil && !errdefs.IsNotFound(err) {
			cancel()
			return fmt.Errorf("stopping workload task: %w", err)
		}
		select {
		case <-wait:
		case <-graceCtx.Done():
		}
	}
	cancel()

	// The grace context may have expired. Use a fresh bounded context to
	// force-delete the task, rather than making clean-up impossible.
	deleteCtx, deleteCancel := context.WithTimeout(context.WithoutCancel(ctx), stopDelay)
	defer deleteCancel()
	_, err = task.Delete(deleteCtx, client.WithProcessKill)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("deleting workload task: %w", err)
	}
	return nil
}
