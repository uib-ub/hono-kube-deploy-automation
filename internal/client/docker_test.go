package client

import (
	"context"
	"os"
	"testing"
)

// Test cases
var dockerTestCases = []struct {
	imageName        string
	registryOwner    string
	imageTag         string
	localRepoSrcPath string
	repo             string
	branch           string
	dockerOptions    *DockerOptions
}{
	{
		imageName:        "uib-ub/uib-ub-monorepo-api",
		registryOwner:    "uib-ub",
		imageTag:         "test",
		localRepoSrcPath: os.Getenv("LOCAL_REPO_SRC"),
		repo:             "uib-ub/uib-ub-monorepo",
		branch:           "main",
		dockerOptions: &DockerOptions{
			ContainerRegistry: "ghcr.io",
			RegistryPassword:  os.Getenv("GITHUB_TOKEN"),
			Dockerfile:        "Dockerfile.api",
		},
	},
}

func TestImageLifecycle(t *testing.T) {
	for i, tc := range dockerTestCases {
		githubCli := NewGithubClient(tc.dockerOptions.RegistryPassword)
		err := githubCli.DownloadGithubRepository(tc.localRepoSrcPath, tc.repo, tc.branch)
		if err != nil {
			t.Errorf("failed to download repository in test case %d: expected nil, got %v", i, err)
		}

		dockerClient, err := NewDockerClient(tc.dockerOptions)
		if err != nil {
			t.Errorf("failed to create docker client in test case %d: expected nil, got %v", i, err)
		}

		t.Run("ImageBuild", func(t *testing.T) {
			err = dockerClient.ImageBuild(
				tc.registryOwner,
				tc.imageName,
				tc.imageTag,
				tc.localRepoSrcPath,
			)
			if err != nil {
				t.Errorf("failed to build image in test case %d: expected nil, got %v", i, err)
			}
		})

		t.Run("ImagePush", func(t *testing.T) {
			err = dockerClient.ImagePush(tc.registryOwner, tc.imageName, tc.imageTag)
			if err != nil {
				t.Errorf("failed to push image in test case %d: expected nil, got %v", i, err)
			}

			t.Cleanup(func() {

				err := githubCli.DeletePackageImage(context.Background(), tc.registryOwner, "container", tc.imageName, tc.imageTag)
				if err != nil {
					t.Errorf("failed to clean up by deleting image on Github packages in test case %d: expected nil, got %v", i, err)
				}
			})
		})

		t.Run("ImageDelete", func(t *testing.T) {
			err = dockerClient.ImageDelete(tc.registryOwner, tc.imageName, tc.imageTag)
			if err != nil {
				t.Errorf("failed to delete image in test case %d: expected nil, got %v", i, err)
			}
		})

		t.Cleanup(func() {
			err := githubCli.DeleteLocalRepository(tc.localRepoSrcPath)
			if err != nil {
				t.Errorf("failed to clean up by deleting local repository in test case %d: expected nil, got %v", i, err)
			}
		})
	}
}
