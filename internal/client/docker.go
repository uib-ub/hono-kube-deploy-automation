package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	log "github.com/sirupsen/logrus"
)

// DockerOptions is a struct that holds the options for the Docker API client operations.
type DockerOptions struct {
	ContainerRegistry string // the registry where the image will be pushed
	RegistryPassword  string // the password for the registry
	Dockerfile        string // the Dockerfile to use for building the image
}

// DockerClient warps the Docker API client and the options for Docker operations.
type DockerClient struct {
	Client        *dockercli.Client // Client is the Docker API client
	DockerOptions *DockerOptions    // DockerOptions holds the configuration options for Docker operations.
}

// NewDockerClient creates a new Docker client with the given options.
func NewDockerClient(dockerOptions *DockerOptions) (*DockerClient, error) {
	client, err := dockercli.NewClientWithOpts(
		dockercli.FromEnv,
		dockercli.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerClient{client, dockerOptions}, nil
}

// ImageBuild builds a Docker image from the given local repository path and tags it.
func (d *DockerClient) ImageBuild(
	registryOwner,
	imageName,
	imageTag,
	localRepoPath string,
) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf(
		"%s/%s/%s:%s",
		containerRegistry,
		registryOwner,
		imageName,
		imageTag,
	)

	// Create a tar archive of the local repository path.
	tar, err := archive.TarWithOptions(localRepoPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Define options for building the image.
	buildOptions := types.ImageBuildOptions{
		Dockerfile:  d.DockerOptions.Dockerfile,
		Tags:        []string{registryNameWithTag},
		Remove:      true,
		ForceRemove: true,
	}

	log.Infof("Building image: %s", registryNameWithTag)
	// Build the image
	buildRes, err := d.Client.ImageBuild(context.Background(), tar, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer buildRes.Body.Close()

	// Stream the build output to the console.
	io.Copy(os.Stdout, buildRes.Body)

	log.Infof("Image %s is built locally", registryNameWithTag)
	return nil
}

// ImagePush pushes the image to the container registry.
func (d *DockerClient) ImagePush(registryOwner, imageName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf(
		"%s/%s/%s:%s",
		containerRegistry,
		registryOwner,
		imageName,
		imageTag,
	)

	registryPassword := d.DockerOptions.RegistryPassword

	// Encode authentication configuration for the registry.
	authConfig := registry.AuthConfig{
		Username: registryOwner,
		Password: registryPassword,
	}
	encodedAuthConfig, err := json.Marshal(authConfig)
	if err != nil {
		return fmt.Errorf("failed to encode auth config: %w", err)
	}

	authBase64 := base64.URLEncoding.EncodeToString(encodedAuthConfig)
	pushOptions := image.PushOptions{
		RegistryAuth: authBase64,
	}

	log.Infof("Pushing image: %s", registryNameWithTag)
	// Push the image to the registry.
	pushRes, err := d.Client.ImagePush(context.Background(), registryNameWithTag, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer pushRes.Close()

	// Stream the push output to the console.
	io.Copy(os.Stdout, pushRes)

	log.Infof("Image %s is pushed to the container registry", registryNameWithTag)
	return nil
}

// ImageDelete deletes a Docker image from the local system and prunes dangling images.
func (d *DockerClient) ImageDelete(registryOwner, imageName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf(
		"%s/%s/%s:%s",
		containerRegistry,
		registryOwner,
		imageName,
		imageTag,
	)

	removeOptions := image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	}

	log.Infof("Deleting image: %s", registryNameWithTag)
	// Remove the image.
	_, err := d.Client.ImageRemove(context.Background(), registryNameWithTag, removeOptions)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	// Prune dangling images to free up space.
	if err := d.pruneDanglingImages(); err != nil {
		return err
	}

	log.Infof("Image %s is deleted locally", registryNameWithTag)
	return nil
}

// pruneDanglingImages removes dangling images from the local system to free up space.
func (d *DockerClient) pruneDanglingImages() error {
	// Set up filter to only target dangling images
	// 'Dangling' images are those tagged with <none>
	pruneFilters := filters.NewArgs()
	pruneFilters.Add("dangling", "true") // This targets only untagged images

	// Execute the prune operation
	report, err := d.Client.ImagesPrune(context.Background(), pruneFilters)
	if err != nil {
		return fmt.Errorf("failed to prune dangling images: %w", err)
	}
	// Log the total space reclaimed by pruning
	log.Infof("Pruned dangling Docker images, reclaimed %d bytes", report.SpaceReclaimed)

	// Log details of pruned images for verification
	for _, image := range report.ImagesDeleted {
		if image.Untagged != "" {
			log.Infof("Untagged image pruned: %s", image.Untagged)
		}
		if image.Deleted != "" {
			log.Infof("Deleted image ID: %s", image.Deleted)
		}
	}

	return nil
}
