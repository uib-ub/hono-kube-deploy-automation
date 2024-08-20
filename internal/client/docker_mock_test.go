package client

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock DockerClient
type MockDockerClient struct {
	mock.Mock
}

// Mock methods for DockerClient interface
func (m *MockDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	args := m.Called(ctx, buildContext, options)
	return args.Get(0).(types.ImageBuildResponse), args.Error(1)
}

func (m *MockDockerClient) ImagePush(ctx context.Context, image string, options image.PushOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, image, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	args := m.Called(ctx, imageID, options)
	return args.Get(0).([]image.DeleteResponse), nil
}

func (m *MockDockerClient) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (image.PruneReport, error) {
	args := m.Called(ctx, pruneFilters)
	return args.Get(0).(image.PruneReport), args.Error(1)
}

var dockerMockTestCases = []struct {
	imageName        string
	registryOwner    string
	imageTag         string
	localRepoSrcPath string
	repo             string
	branch           string
	dockerOptions    *DockerOptions
}{
	{
		imageName:        "test-owner/test-repo-api",
		registryOwner:    "test-owner",
		imageTag:         "test",
		localRepoSrcPath: "./test-repo",
		repo:             "test-owner/test-repo",
		branch:           "main",
		dockerOptions: &DockerOptions{
			ContainerRegistry: "test-registry",
			RegistryPassword:  "test-passwor",
			Dockerfile:        "Dockerfile",
		},
	},
}

func TestDockerMockLifecycle(t *testing.T) {
	for _, tc := range dockerMockTestCases {
		mockDocker := new(MockDockerClient)
		// mockTarFunc is a mock implementation of the TarWithOptionsFunc type.
		// It simulates the creation of a tarball by returning a simple io.ReadCloser
		// containing "mocked tarball content". This allows us to test the ImageBuild
		// method without actually creating a tarball.
		mockTarFunc := func(srcPath string, options *archive.TarOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("mocked tarball content")), nil
		}
		// Create a DockerClient instance using the mock Docker client and mock tarball function.
		dockerClient := &DockerClient{
			Client:         mockDocker,
			DockerOptions:  tc.dockerOptions,
			TarWithOptions: mockTarFunc, // Inject the mock tarball creation function.
		}

		t.Run("ImageBuild", func(t *testing.T) {
			// Setup mock response for a successful image build.
			// The Docker client will return a response with "Build successful" as the body.
			mockDocker.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything).Return(types.ImageBuildResponse{
				Body: io.NopCloser(strings.NewReader("Build successful")),
			}, nil)

			// Call the ImageBuild method with the mocked tarball and check that it succeeds.
			err := dockerClient.ImageBuild(tc.registryOwner, tc.imageName, tc.imageTag, tc.localRepoSrcPath)
			assert.NoError(t, err, "expected no error from ImageBuild")

			// Verify that the mock Docker client was called as expected.
			mockDocker.AssertExpectations(t)
		})

		t.Run("ImagePush", func(t *testing.T) {
			// Setup mock response
			mockDocker.On("ImagePush", mock.Anything, mock.Anything, mock.Anything).Return(io.NopCloser(
				strings.NewReader("Push successful")), nil)

			err := dockerClient.ImagePush(tc.registryOwner, tc.imageName, tc.imageTag)
			assert.NoError(t, err, "expected no error from ImagePush")

			mockDocker.AssertExpectations(t)
		})

		t.Run("ImageDelete", func(t *testing.T) {
			// Setup mock responses
			mockDocker.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything).Return([]image.DeleteResponse{}, nil)
			mockDocker.On("ImagesPrune", mock.Anything, mock.Anything).Return(image.PruneReport{
				SpaceReclaimed: 100,
				ImagesDeleted: []image.DeleteResponse{
					{
						Untagged: "untagged-image-id",
						Deleted:  "deleted-image-id",
					},
				},
			}, nil)

			err := dockerClient.ImageDelete(tc.registryOwner, tc.imageName, tc.imageTag)
			assert.NoError(t, err, "expected no error from ImageDelete")

			mockDocker.AssertExpectations(t)
		})
	}
}
