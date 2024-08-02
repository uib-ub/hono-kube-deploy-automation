package client

import (
	"os"
	"testing"
)

// Test cases
var dockerTestCases = []struct {
	imageName        string
	registryOwner    string
	imageTag         string
	localRepoSrcPath string
	dockerOptions    *DockerOptions
}{
	{
		imageName:        "uib-ub/uib-ub-monorepo-api",
		registryOwner:    "uib-ub",
		imageTag:         "test",
		localRepoSrcPath: os.Getenv("LOCAL_REPO_SRC"),
		dockerOptions: &DockerOptions{
			ContainerRegistry: "ghcr.io",
			RegistryPassword:  os.Getenv("GITHUB_TOKEN"),
			Dockerfile:        "Dockerfile.api",
		},
	},
}

func TestImageBuild(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Errorf("failed to create docker client in test case %d: expected nil, got %v", i, err)
		}

		err = dockerClient.ImageBuild(tc.registryOwner, tc.imageName, tc.imageTag, tc.localRepoSrcPath)
		if err != nil {
			t.Errorf("failed to build image in test case %d: expected nil, got %v", i, err)
		}
	}
}

func TestImagePush(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Errorf("failed to create docker client in test case %d: expected nil, got %v", i, err)
		}
		err = dockerClient.ImagePush(tc.registryOwner, tc.imageName, tc.imageTag)
		if err != nil {
			t.Errorf("failed to push image in test case %d: expected nil, got %v", i, err)
		}
	}
}

func TestImageDelete(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Errorf("failed to create docker client in test case %d: expected nil, got %v", i, err)
		}
		err = dockerClient.ImageDelete(tc.registryOwner, tc.imageName, tc.imageTag)
		if err != nil {
			t.Errorf("failed to delete image in test case %d: expected nil, got %v", i, err)
		}
	}
}
