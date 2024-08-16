package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var newKustomizerTestCases = []struct {
	name     string
	kubeSrc  string
	expected string
}{
	{
		name:     "Test creating new kustomizer instance with valid path",
		kubeSrc:  "some/source/test/path",
		expected: "some/source/test/path",
	},
	{
		name:     "Test creating new kustomizer instance with empty path",
		kubeSrc:  "",
		expected: "",
	},
}

func TestNewKustomizer(t *testing.T) {

	for _, tc := range newKustomizerTestCases {
		t.Run(tc.name, func(t *testing.T) {
			kustomizer := NewKustomizer(tc.kubeSrc)
			assert.NotNil(t, kustomizer, "Expected NewKustomizer to return a non-nil Kustomizer")
			assert.Equal(t, tc.expected, kustomizer.KubeSrc, "Expected KubeSrc to be set correctly")
		})
	}
}

var kustomizerBuildTestCases = []struct {
	name            string
	setupFunc       func(t *testing.T) string
	expectedContent string
	expectedError   bool
}{
	{
		name: "Valid kustomization directory with one resource",
		setupFunc: func(t *testing.T) string {
			dir, err := os.MkdirTemp("", "kustomization_test")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}

			kustomizationContent := `
resources:
- deployment.yaml
`
			deploymentContent := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

			err = os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomizationContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write kustomization.yaml: %v", err)
			}

			err = os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(deploymentContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write deployment.yaml: %v", err)
			}

			return dir
		},
		expectedContent: "kind: Deployment",
		expectedError:   false,
	},
	{
		name: "Empty kustomization directory",
		setupFunc: func(t *testing.T) string {
			dir, err := os.MkdirTemp("", "empty_kustomization_test")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			return dir
		},
		expectedContent: "",
		expectedError:   true,
	},
	{
		name: "Invalid kustomization directory",
		setupFunc: func(t *testing.T) string {
			return "invalid/directory/path"
		},
		expectedContent: "",
		expectedError:   true,
	},
}

func TestKustomizerBuild(t *testing.T) {
	for _, tc := range kustomizerBuildTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the kustomization directory for this test case
			kustomizationDir := tc.setupFunc(t)
			if kustomizationDir != "invalid/directory/path" {
				defer os.RemoveAll(kustomizationDir) // Clean up after test
			}

			kustomizer := NewKustomizer(kustomizationDir)
			result, err := kustomizer.Build()

			if tc.expectedError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error from Build but got one")
				assert.NotNil(t, result, "Expected Build to return non-nil result")
				assert.NotEmpty(t, result, "Expected Build to return non-empty result")
				assert.Contains(t, result[0], tc.expectedContent, "Expected result to contain expected content")
			}
		})
	}
}
