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
	ImageNameSuffix   string
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

func (d *DockerClient) ImageBuild(registryOwner, registryFullName, imageTag, localRepoSrcPath string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	imageNameSuffix := d.DockerOptions.ImageNameSuffix
	imageNameWithTag := fmt.Sprintf("%s/%s/%s-%s:%s", containerRegistry, registryOwner, registryFullName, imageNameSuffix, imageTag)

	tar, err := archive.TarWithOptions(localRepoSrcPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	buildOptions := types.ImageBuildOptions{
		Dockerfile:  d.DockerOptions.Dockerfile,
		Tags:        []string{imageNameWithTag},
		Remove:      true,
		ForceRemove: true,
	}
	log.Infof("Building image: %s", imageNameWithTag)
	buildRes, err := d.Client.ImageBuild(context.Background(), tar, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer buildRes.Body.Close()

	io.Copy(os.Stdout, buildRes.Body)

	log.Infof("Image %s is built locally", imageNameWithTag)
	return nil
}

func (d *DockerClient) ImagePush(registryOwner, registryFullName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	imageNameSuffix := d.DockerOptions.ImageNameSuffix
	imageNameWithTag := fmt.Sprintf("%s/%s/%s-%s:%s", containerRegistry, registryOwner, registryFullName, imageNameSuffix, imageTag)
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

	log.Infof("Pushing image: %s", imageNameWithTag)
	pushRes, err := d.Client.ImagePush(context.Background(), imageNameWithTag, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer pushRes.Close()

	io.Copy(os.Stdout, pushRes)

	log.Infof("Image %s is pushed to the container registry", imageNameWithTag)
	return nil
}

func (d *DockerClient) ImageDelete(registryOwner, registryFullName, imageTag string) error {
	containerRegistry := d.DockerOptions.ContainerRegistry
	imageNameSuffix := d.DockerOptions.ImageNameSuffix
	imageNameWithTag := fmt.Sprintf("%s/%s/%s-%s:%s", containerRegistry, registryOwner, registryFullName, imageNameSuffix, imageTag)

	removeOptions := image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	}

	log.Infof("Deleting image: %s", imageNameWithTag)
	_, err := d.Client.ImageRemove(context.Background(), imageNameWithTag, removeOptions)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	log.Infof("Image %s is deleted locally", imageNameWithTag)
	return nil
}
