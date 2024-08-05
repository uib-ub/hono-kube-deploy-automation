package config

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

const (
	defaultLocalRepoSrc = "app"                     // default local repository source folder
	defaultDockerFile   = "Dockerfile"              // default Dockerfile
	defaultRegistry     = "ghcr.io"                 // default container registry
	defaultKubeResSrc   = "microk8s-hono-api"       // default Kubernetes resource source folder
	defaultWFPrefix     = "deploy-hono-api-secrets" // default Github workflow file name prefix
	defaultImageSuffix  = "api"                     // default Docker Image name suffix
)

type Config struct {
	GitHubToken   string
	WebhookSecret string
	KubeConfig    string // Kubernetes config file
	LocalRepoDir  string // local repository path
	DockerFile    string // DockerFile
	Registry      string // container registry
	KubeResSrc    string // Kubernetes resource source folder
	WFPrefix      string // Github workflow file name prefix
	ImageSuffix   string // Docker Image name suffix
}

func LoadConfig() (*Config, error) {

	localRepoDir, err := getLocalRepoPath(getEnv("LOCAL_REPO_SRC", defaultLocalRepoSrc))
	if err != nil {
		return nil, err
	}

	config := &Config{
		GitHubToken:   getEnv("GITHUB_TOKEN", ""),
		WebhookSecret: getEnv("WEBHOOK_SECRET", ""),
		KubeConfig:    getEnv("KUBECONFIG", ""),
		LocalRepoDir:  localRepoDir,
		DockerFile:    getEnv("DOCKERFILE", defaultDockerFile),
		Registry:      getEnv("CONTAINER_REGISTRY", defaultRegistry),
		KubeResSrc:    getEnv("KUBE_RESOURCE", defaultKubeResSrc),
		WFPrefix:      getEnv("WORKFLOW_PREFIX", defaultWFPrefix),
		ImageSuffix:   getEnv("IMAGE_SUFFIX", defaultImageSuffix),
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

func getLocalRepoPath(localRepoSrcFolder string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	log.Infof("Home directory: %s", homeDir)
	return filepath.Join(homeDir, localRepoSrcFolder), nil
}
