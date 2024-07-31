package client

import (
	"fmt"

	dockercli "github.com/docker/docker/client"
)

type DockerOptions struct {
	ContainerRegistry         string
	ContainerRegistryPassword string
	Dockerfile                string
}

type DockerClient struct {
	*dockercli.Client
	DockerOptions *DockerOptions
}

func NewDockerClient(dockerOptions *DockerOptions) (*DockerClient, error) {
	client, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerClient{client, dockerOptions}, nil
}
