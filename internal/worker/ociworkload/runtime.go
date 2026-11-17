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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/juju/names/v6"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	unitLabel      = "com.canonical.juju.unit"
	containerLabel = "com.canonical.juju.container"
	imageLabel     = "com.canonical.juju.image"
	specLabel      = "com.canonical.juju.spec"
	stopDelay      = 10 * time.Second

	// charmLayoutSubdirs hold the paths a Kubernetes charm container
	// presents below its root: charm code, notably the ops framework,
	// resolves every workload's Pebble socket as
	// <root>/containers/<name>/pebble.socket and the Pebble binary as
	// <root>/bin/pebble.
	charmLayoutContainersSubdir = "containers"
	charmLayoutBinSubdir        = "bin"
	pebbleSocketName            = "pebble.socket"
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
	// ContainersDir holds the unit's shared socket directory per container:
	// the Pebble socket and the persistent Pebble state seeded from the
	// image by PEBBLE_COPY_ONCE, like a Kubernetes charm volume. StateDir
	// holds workload task logs.
	ContainersDir string
	StateDir      string
	// UID and GID override the image's user when supplied by charm metadata.
	UID *int
	GID *int
}

// ContainerRuntime converges each declared workload to a running Pebble task.
// Ensure is idempotent across worker and machine restarts, and mirrors the
// Kubernetes charm container layout on the host. RemoveContainers stops the
// unit's containers except those kept, so charm metadata changes retire the
// workloads it no longer declares; an empty keep set stops all of them.
// Persistent Pebble state is retained either way, so temporary agent
// restarts do not lose the charm's service plan.
type ContainerRuntime interface {
	Ensure(context.Context, ContainerSpec) error
	RemoveContainers(context.Context, string, []string) error
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
	// CharmLayoutDir is the root at which the runtime mirrors the paths a
	// Kubernetes charm container presents, so unmodified sidecar charms
	// resolve their workload sockets and the Pebble binary. The machine
	// manifold supplies /charm; an empty root disables the mirror. The
	// layout is per-machine like the host, not per-pod, so two units on
	// one machine cannot declare same-named containers.
	CharmLayoutDir string
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
	if err := r.mirrorCharmContainerLayout(spec); err != nil {
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
	return r.RemoveContainers(ctx, unitName, nil)
}

// RemoveContainers stops every task owned by the unit except those naming a
// kept container, so containers no longer present in the current charm
// metadata are removed while upgrades retain the rest. The mirrored
// /charm/containers layout follows the unit's containers.
func (r ContainerdRuntime) RemoveContainers(ctx context.Context, unitName string, keep []string) error {
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
	keepSet := make(map[string]bool, len(keep))
	for _, name := range keep {
		keepSet[name] = true
	}
	for _, container := range containers {
		labels, err := container.Labels(ctx)
		if err != nil {
			return fmt.Errorf("reading workload ownership: %w", err)
		}
		if labels[unitLabel] != unitName || keepSet[labels[containerLabel]] {
			continue
		}
		if err := removeContainer(ctx, container); err != nil {
			return err
		}
		if err := r.removeCharmContainerMirror(names.NewUnitTag(unitName).String(), labels[containerLabel]); err != nil {
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
		unitLabel:      spec.UnitName,
		containerLabel: spec.Name,
		imageLabel:     imageDigest,
		specLabel:      hex.EncodeToString(digest[:]),
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
		filepath.Join(spec.StateDir, spec.Name),
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

// mirrorCharmContainerLayout makes the machine host present the paths a
// Kubernetes charm container would: the workload's Pebble socket at
// <CharmLayoutDir>/containers/<name>/pebble.socket, resolved directly by
// charm code, and the Pebble binary at <CharmLayoutDir>/bin/pebble. The
// mirror stays private to the machine agent, like the socket directory it
// points into.
func (r ContainerdRuntime) mirrorCharmContainerLayout(spec ContainerSpec) error {
	if r.CharmLayoutDir == "" {
		return nil
	}
	socketLink := filepath.Join(r.CharmLayoutDir, charmLayoutContainersSubdir, spec.Name, pebbleSocketName)
	socketTarget := filepath.Join(spec.ContainersDir, spec.Name, pebbleSocketName)
	if err := ensureOwnedSymlink(socketLink, socketTarget); err != nil {
		return fmt.Errorf("mirroring Pebble socket for container %q: %w", spec.Name, err)
	}

	// The Pebble binary is identical for every unit on the machine, so an
	// existing mirror is tolerated rather than treated as a conflict.
	pebbleLink := filepath.Join(r.CharmLayoutDir, charmLayoutBinSubdir, "pebble")
	if err := ensureSymlink(pebbleLink, spec.PebbleBinary); err != nil {
		return fmt.Errorf("mirroring Pebble binary: %w", err)
	}
	return nil
}

// ensureSymlink points link at target, creating it if absent. An existing
// entry is left alone: the mirrored Pebble binary is identical for every
// unit, and repointing it would serve no one.
func ensureSymlink(link, target string) error {
	if _, err := os.Readlink(link); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		return fmt.Errorf("creating mirror parent %q: %w", filepath.Dir(link), err)
	}
	if err := os.Symlink(target, link); err != nil {
		// Another unit mirrored first; the binaries are identical.
		if _, err := os.Readlink(link); err == nil {
			return nil
		}
		return fmt.Errorf("creating mirror %q: %w", link, err)
	}
	return nil
}

// ensureOwnedSymlink points link at target, creating it if absent. Unlike
// ensureSymlink, an existing entry pointing elsewhere is an error: a socket
// mirror belongs to exactly one unit, and silently repointing it would
// connect another unit's charm to this unit's workload.
func ensureOwnedSymlink(link, target string) error {
	switch existing, err := os.Readlink(link); {
	case err == nil:
		if existing != target {
			return fmt.Errorf("mirror %q already points at %q", link, existing)
		}
		return nil
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
			return fmt.Errorf("creating mirror parent %q: %w", filepath.Dir(link), err)
		}
		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("creating mirror %q: %w", link, err)
		}
		return nil
	default:
		// The path exists but is not a symlink.
		return fmt.Errorf("mirror %q exists and is not a symlink", link)
	}
}

// removeCharmContainerMirror removes the mirrored socket link of the named
// container of the named unit. Links pointing into another unit's socket
// directory are left alone, so same-named containers of other units keep
// their mirrors.
func (r ContainerdRuntime) removeCharmContainerMirror(unitTag, containerName string) error {
	if r.CharmLayoutDir == "" {
		return nil
	}
	link := filepath.Join(r.CharmLayoutDir, charmLayoutContainersSubdir, containerName, pebbleSocketName)
	target, err := os.Readlink(link)
	if err != nil {
		// Absent, or foreign content this unit never owned.
		return nil
	}
	if !strings.Contains(target, filepath.Join("agents", unitTag, "containers")) {
		return nil
	}
	if err := os.RemoveAll(filepath.Join(r.CharmLayoutDir, charmLayoutContainersSubdir, containerName)); err != nil {
		return fmt.Errorf("removing Pebble socket mirror for container %q: %w", containerName, err)
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
//
// Like a Kubernetes workload container, nothing is mounted at
// /var/lib/pebble: images bake their Pebble layers there, and
// PEBBLE_COPY_ONCE seeds them into $PEBBLE, which points into the mounted
// socket directory and so persists on the host.
func pebbleSpecOptions(spec ContainerSpec) []oci.SpecOpts {
	mounts := []specs.Mount{
		{Destination: "/charm/bin/pebble", Type: "bind", Source: spec.PebbleBinary, Options: []string{"rbind", "ro"}},
		{Destination: "/charm/container", Type: "bind", Source: filepath.Join(spec.ContainersDir, spec.Name), Options: []string{"rbind", "rw"}},
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
