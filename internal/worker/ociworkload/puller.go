// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/client"
	"github.com/distribution/reference"
	"github.com/juju/errors"

	jujudocker "github.com/juju/juju/internal/docker"
)

const namespace = "juju"

// Puller downloads OCI images into the machine's containerd content store.
// It creates and closes a connection for each pull, so failed deployments do
// not leave connections behind.
type Puller struct {
	// NewClient is provided by the unit manifold. Each pull closes the
	// connection it creates, including when authentication or download fails.
	NewClient ContainerdClientFactory
}

// Pull downloads and unpacks an image, including images from private
// registries. Registry credentials are only supplied to the image's registry
// and are never passed to a command line or stored in containerd.
func (p Puller) Pull(ctx context.Context, image jujudocker.DockerImageDetails) error {
	if err := jujudocker.ValidateDockerRegistryPath(image.RegistryPath); err != nil {
		return errors.Trace(err)
	}
	conn, err := p.NewClient()
	if err != nil {
		return errors.Annotate(err, "connecting to containerd")
	}
	defer conn.Close()

	options := []client.RemoteOpt{client.WithPullUnpack}
	if image.IsPrivate() {
		registryImage, err := reference.ParseNormalizedNamed(image.RegistryPath)
		if err != nil {
			return errors.Annotate(err, "parsing OCI image registry")
		}
		registryHost := reference.Domain(registryImage)
		username, secret, err := imageCredentials(image)
		if err != nil {
			return errors.Trace(err)
		}
		// Never pass the unit's registry secret to an unrelated registry,
		// including hosts reached through a redirect.
		resolver := docker.NewResolver(docker.ResolverOptions{
			Credentials: func(host string) (string, string, error) {
				if host != registryHost && !(registryHost == "docker.io" && host == "registry-1.docker.io") {
					return "", "", nil
				}
				return username, secret, nil
			},
		})
		options = append(options, client.WithResolver(resolver))
	}
	if _, err := conn.Pull(ctx, image.RegistryPath, options...); err != nil {
		return errors.Annotatef(err, "pulling OCI image %q", image.RegistryPath)
	}
	return nil
}

func imageCredentials(image jujudocker.DockerImageDetails) (string, string, error) {
	if image.Username != "" || image.Password != "" {
		return image.Username, image.Password, nil
	}
	if !image.Auth.Empty() {
		decoded, err := base64.StdEncoding.DecodeString(image.Auth.Content())
		if err != nil {
			return "", "", errors.Annotate(err, "decoding OCI registry credentials")
		}
		username, secret, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return "", "", errors.New("OCI registry credentials have no separator")
		}
		return username, secret, nil
	}
	if !image.RegistryToken.Empty() {
		return "", image.RegistryToken.Content(), nil
	}
	if !image.IdentityToken.Empty() {
		return "", image.IdentityToken.Content(), nil
	}
	return "", "", nil
}
