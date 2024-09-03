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

// DockerAPIClient defines the methods that your DockerClient will use.
type DockerAPIClient interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options image.PushOptions) (io.ReadCloser, error)
	ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error)
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (image.PruneReport, error)
}

// Ensure that dockercli.Client implements DockerAPIClient
var _ DockerAPIClient = &dockercli.Client{}

// TarWithOptionsFunc is a function type that matches the signature of archive.TarWithOptions.
// It accepts a source path and tar options and returns an io.ReadCloser representing the tarball and an error.
// This allows us to inject different implementations (e.g., for testing) into DockerClient.
type TarWithOptionsFunc func(srcPath string, options *archive.TarOptions) (io.ReadCloser, error)

// DockerClient struct accepts an interface that both *dockercli.Client and MockDockerClient implement.
// This approach allows you to use both real and fake clients interchangeably.
type DockerClient struct {
	// Client *dockercli.Client // Client is the Docker API client
	Client         DockerAPIClient    // Use the interface here
	DockerOptions  *DockerOptions     // DockerOptions holds the configuration options for Docker operations.
	TarWithOptions TarWithOptionsFunc // TarWithOptions is a function used to create tarballs; it can be mocked for testing.
}

// NewDockerClient creates a new Docker client with the given options and tarball creation function.
// If no custom tarball function is provided, the default archive.TarWithOptions function is used.
func NewDockerClient(dockerOptions *DockerOptions, tarFunc TarWithOptionsFunc) (*DockerClient, error) {
	client, err := dockercli.NewClientWithOpts(
		dockercli.FromEnv,
		dockercli.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	// If no custom tarball function is provided, use the default archive.TarWithOptions.
	if tarFunc == nil {
		tarFunc = archive.TarWithOptions
	}
	//return &DockerClient{client, dockerOptions}, nil
	return &DockerClient{
		Client:         client,
		DockerOptions:  dockerOptions,
		TarWithOptions: tarFunc,
	}, nil
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

	// Create a tar archive of the local repository path using the injected TarWithOptions function.
	// This function is either the real archive.TarWithOptions or a mock provided during testing.
	tar, err := d.TarWithOptions(localRepoPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Define options for building the image.
	buildOptions := types.ImageBuildOptions{
		Dockerfile: d.DockerOptions.Dockerfile,
		Tags:       []string{registryNameWithTag},
		//		Remove:      true, // remove intermediate containers created during the build process
		//		ForceRemove: true, // forces the removal of intermediate containers even if the build fails
	}

	log.Infof("Building image: %s", registryNameWithTag)
	// Build the image
	buildRes, err := d.Client.ImageBuild(context.Background(), tar, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}

	if buildRes.Body != nil {
		defer buildRes.Body.Close()
		// Stream the build output to the console.
		io.Copy(os.Stdout, buildRes.Body)
	} else {
		return fmt.Errorf("Build response body is nil for image: %s", registryNameWithTag)
	}

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
