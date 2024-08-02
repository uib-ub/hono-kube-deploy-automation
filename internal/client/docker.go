package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	log "github.com/sirupsen/logrus"
)

type DockerOptions struct {
	ContainerRegistry string
	RegistryPassword  string
	Dockerfile        string
}

type DockerClient struct {
	Client        *dockercli.Client
	DockerOptions *DockerOptions
}

func NewDockerClient(dockerOptions *DockerOptions) (*DockerClient, error) {
	client, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerClient{client, dockerOptions}, nil
}

func (d *DockerClient) ImageBuild(registryOwner, imageName, imageTag, localRepoPath string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf("%s/%s/%s:%s", containerRegistry, registryOwner, imageName, imageTag)

	tar, err := archive.TarWithOptions(localRepoPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	buildOptions := types.ImageBuildOptions{
		Dockerfile:  d.DockerOptions.Dockerfile,
		Tags:        []string{registryNameWithTag},
		Remove:      true,
		ForceRemove: true,
	}
	log.Infof("Building image: %s", registryNameWithTag)
	buildRes, err := d.Client.ImageBuild(context.Background(), tar, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer buildRes.Body.Close()

	io.Copy(os.Stdout, buildRes.Body)

	log.Infof("Image %s is built locally", registryNameWithTag)
	return nil
}

func (d *DockerClient) ImagePush(registryOwner, imageName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf("%s/%s/%s:%s", containerRegistry, registryOwner, imageName, imageTag)
	registryPassword := d.DockerOptions.RegistryPassword

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
	pushRes, err := d.Client.ImagePush(context.Background(), registryNameWithTag, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer pushRes.Close()

	io.Copy(os.Stdout, pushRes)

	log.Infof("Image %s is pushed to the container registry", registryNameWithTag)
	return nil
}

func (d *DockerClient) ImageDelete(registryOwner, imageName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	registryNameWithTag := fmt.Sprintf("%s/%s/%s:%s", containerRegistry, registryOwner, imageName, imageTag)

	removeOptions := image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	}

	log.Infof("Deleting image: %s", registryNameWithTag)
	_, err := d.Client.ImageRemove(context.Background(), registryNameWithTag, removeOptions)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	log.Infof("Image %s is deleted locally", registryNameWithTag)
	return nil
}
