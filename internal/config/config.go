package config

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

const (
	defaultLocalRepoSrcFolder = "app"
	defaultDockerFile         = "Dockerfile"
	defaultContainerRegistry  = "ghcr.io"
	defaultKubeResourcePath   = "microk8s-hono-api"
	defaultWorkFowFilePrefix  = "kube-secrets-deploy"
)

type Config struct {
	GitHubToken        string
	WebhookSecret      string
	KubeConfigFile     string
	LocalRepoSrcPath   string
	DockerFile         string
	ContainerRegistry  string
	KubeResourcePath   string
	WorkFlowFilePrefix string
}

func LoadConfig() (*Config, error) {

	localRepoSrcPath, err := getLocalRepoSrcPath(getEnv("LOCAL_REPO_SRC", defaultLocalRepoSrcFolder))
	if err != nil {
		return nil, err
	}

	config := &Config{
		GitHubToken:        getEnv("GITHUB_TOKEN", ""),
		WebhookSecret:      getEnv("WEBHOOK_SECRET", ""),
		KubeConfigFile:     getEnv("KUBECONFIG", ""),
		LocalRepoSrcPath:   localRepoSrcPath,
		DockerFile:         getEnv("DOCKER_FILE", defaultDockerFile),
		ContainerRegistry:  getEnv("CONTAINER_REGISTRY", defaultContainerRegistry),
		KubeResourcePath:   getEnv("KUBE_RESOURCE_PATH", defaultKubeResourcePath),
		WorkFlowFilePrefix: getEnv("WORKFLOW_FILE_PREFIX", defaultWorkFowFilePrefix),
	}

	if config.WebhookSecret == "" || config.GitHubToken == "" {
		return nil, fmt.Errorf("missing required configuration")
	}

	return config, nil
}

// getEnv fetches an environment variable or returns a default value if not set.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getLocalRepoSrcPath(localRepoSrcFolder string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	log.Infof("Home directory: %s", homeDir)
	return filepath.Join(homeDir, localRepoSrcFolder), nil
}
