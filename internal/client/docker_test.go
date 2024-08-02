package client

import (
	"os"
	"testing"
)

// Test cases
var dockerTestCases = []struct {
	repoFullName     string
	registryOwner    string
	imageTag         string
	localRepoSrcPath string
	dockerOptions    *DockerOptions
}{
	{
		repoFullName:     "uib-ub/uib-ub-monorepo",
		registryOwner:    "uib-ub",
		imageTag:         "test",
		localRepoSrcPath: os.Getenv("LOCAL_REPO_SRC"),
		dockerOptions: &DockerOptions{
			ContainerRegistry: "ghcr.io",
			RegistryPassword:  os.Getenv("GITHUB_TOKEN"),
			Dockerfile:        "Dockerfile.api",
			ImageNameSuffix:   "api",
		},
	},
}

func TestImageBuild(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Fatalf("failed to create docker client in test case %d: %v", i, err)
		}

		err = dockerClient.ImageBuild(tc.registryOwner, tc.repoFullName, tc.imageTag, tc.localRepoSrcPath)
		if err != nil {
			t.Fatalf("failed to build image in test case %d: %v", i, err)
		}
	}
}

func TestImagePush(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Fatalf("failed to create docker client in test case %d: %v", i, err)
		}
		err = dockerClient.ImagePush(tc.registryOwner, tc.repoFullName, tc.imageTag)
		if err != nil {
			t.Fatalf("failed to push image in test case %d: %v", i, err)
		}
	}
}

func TestImageDelete(t *testing.T) {
	for i, tc := range dockerTestCases {
		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Fatalf("failed to create docker client in test case %d: %v", i, err)
		}
		err = dockerClient.ImageDelete(tc.registryOwner, tc.repoFullName, tc.imageTag)
		if err != nil {
			t.Fatalf("failed to delete image in test case %d: %v", i, err)
		}
	}
}
