package client

import (
	"os"
	"testing"
)

var (
	repoFullName     = "uib-ub/uib-ub-monorepo"
	registryOwner    = "uib-ub"
	imageTag         = "test"
	localRepoSrcPath = os.Getenv("LOCAL_REPO_SRC")
)

var dockerOptions = DockerOptions{
	ContainerRegistry: "ghcr.io",
	RegistryPassword:  os.Getenv("GITHUB_TOKEN"),
	Dockerfile:        "Dockerfile.api",
	ImageNameSuffix:   "api",
}

func TestImageBuild(t *testing.T) {
	dockerClient, err := NewDockerClient(&dockerOptions)
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	err = dockerClient.ImageBuild(registryOwner, repoFullName, imageTag, localRepoSrcPath)
	if err != nil {
		t.Fatalf("failed to build image: %v", err)
	}
}

func TestImagePush(t *testing.T) {
	dockerClient, err := NewDockerClient(&dockerOptions)
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	err = dockerClient.ImagePush(registryOwner, repoFullName, imageTag)
	if err != nil {
		t.Fatalf("failed to push image: %v", err)
	}
}

func TestImageDelete(t *testing.T) {
	dockerClient, err := NewDockerClient(&dockerOptions)
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	err = dockerClient.ImageDelete(registryOwner, repoFullName, imageTag)
	if err != nil {
		t.Fatalf("failed to delete image: %v", err)
	}
}
