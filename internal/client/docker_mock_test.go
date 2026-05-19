package client

import (
	"context"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	dockercli "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// stubPushResponse implements dockercli.ImagePushResponse over an io.ReadCloser
// for tests; JSONMessages yields nothing and Wait is a no-op.
type stubPushResponse struct {
	io.ReadCloser
}

func (stubPushResponse) JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error] {
	return func(yield func(jsonstream.Message, error) bool) {}
}

func (stubPushResponse) Wait(ctx context.Context) error { return nil }

// Mock DockerClient
type MockDockerClient struct {
	mock.Mock
}

// Mock methods for DockerClient interface
func (m *MockDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options dockercli.ImageBuildOptions) (dockercli.ImageBuildResult, error) {
	args := m.Called(ctx, buildContext, options)
	return args.Get(0).(dockercli.ImageBuildResult), args.Error(1)
}

func (m *MockDockerClient) ImagePush(ctx context.Context, image string, options dockercli.ImagePushOptions) (dockercli.ImagePushResponse, error) {
	args := m.Called(ctx, image, options)
	return args.Get(0).(dockercli.ImagePushResponse), args.Error(1)
}

func (m *MockDockerClient) ImageRemove(ctx context.Context, imageID string, options dockercli.ImageRemoveOptions) (dockercli.ImageRemoveResult, error) {
	args := m.Called(ctx, imageID, options)
	return args.Get(0).(dockercli.ImageRemoveResult), args.Error(1)
}

func (m *MockDockerClient) ImagePrune(ctx context.Context, opts dockercli.ImagePruneOptions) (dockercli.ImagePruneResult, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(dockercli.ImagePruneResult), args.Error(1)
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

		t.Run("NewDockerClient", func(t *testing.T) {
			dockerClient, err := NewDockerClient(tc.dockerOptions, nil)

			assert.NoError(t, err, "expected no error when creating DockerClient with default tarball function")
			assert.NotNil(t, dockerClient, "expected DockerClient to be non-nil")

			// Verify that calling the TarWithOptions function behaves as expected
			tarball, err := dockerClient.TarWithOptions(tc.localRepoSrcPath, &archive.TarOptions{})
			assert.NoError(t, err, "expected no error when calling TarWithOptions")
			assert.NotNil(t, tarball, "expected TarWithOptions to return a non-nil tarball")
		})

		t.Run("ImageBuild", func(t *testing.T) {
			// Setup mock response for a successful image build.
			// The Docker client will return a response with "Build successful" as the body.
			mockDocker.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything).Return(dockercli.ImageBuildResult{
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
			pushResp := stubPushResponse{ReadCloser: io.NopCloser(strings.NewReader("Push successful"))}
			mockDocker.On("ImagePush", mock.Anything, mock.Anything, mock.Anything).Return(
				dockercli.ImagePushResponse(pushResp), nil)

			err := dockerClient.ImagePush(tc.registryOwner, tc.imageName, tc.imageTag)
			assert.NoError(t, err, "expected no error from ImagePush")

			mockDocker.AssertExpectations(t)
		})

		t.Run("ImageDelete", func(t *testing.T) {
			// Setup mock responses
			mockDocker.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything).Return(
				dockercli.ImageRemoveResult{Items: []image.DeleteResponse{}}, nil)
			mockDocker.On("ImagePrune", mock.Anything, mock.Anything).Return(dockercli.ImagePruneResult{
				Report: image.PruneReport{
					SpaceReclaimed: 100,
					ImagesDeleted: []image.DeleteResponse{
						{
							Untagged: "untagged-image-id",
							Deleted:  "deleted-image-id",
						},
					},
				},
			}, nil)

			err := dockerClient.ImageDelete(tc.registryOwner, tc.imageName, tc.imageTag)
			assert.NoError(t, err, "expected no error from ImageDelete")

			mockDocker.AssertExpectations(t)
		})
	}
}
