package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
)

// Config holds the configuration for the application
type Config struct {
	RollbarToken  string           // Rollbar access token
	GitHubToken   string           // the Github personal access token
	WebhookSecret string           // the webhook secret key
	KubeConfig    string           // the path to the Kubernetes configuration file
	Github        GithubConfig     // Github holds the GitHub-specific configuration settings.
	Kubernetes    KubernetesConfig // Kubernetes holds the Kubernetes-specific configuration settings.
	Container     ContainerConfig  // Container holds the container-related configuration settings.
}

// GithubConfig holds GitHub specific configuration
type GithubConfig struct {
	WorkflowPrefix string // the prefix used for naming workflows in GitHub Actions
	LocalRepo      string // the path to the local repository used for GitHub operations
	PackageType    string
}

// KubernetesConfig holds Kubernetes specific configuration
type KubernetesConfig struct {
	Resource      string // the directory for the Kubernetes resource files
	DevNamespace  string // the namespace used for development environments in Kubernetes.
	TestNamespace string // the namespace used for testing environments in Kubernetes.
}

// ContainerConfig holds container specific configuration
type ContainerConfig struct {
	Registry    string // the container registry name where images are pushed.
	Dockerfile  string // the Dockerfile name used to build the container image.
	ImageSuffix string // the suffix used for naming container images.
}

// Constants for the configuration file's location and type
const (
	configPath = "./internal/config" // Path to the config directory.
	configName = "config"            // Name of the config file
	configType = "yaml"              // Config file type (YAML).
)

// NewConfig initializes and returns a new Config instance by reading the configuration file
// and binding environment variables. It also sets up a watch on the configuration file
// for any changes.
func NewConfig() (*Config, error) {
	// Set the base properties of Viper to locate and read the config file.
	viper.SetConfigType(configType)
	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	// Automatically use environment variables where available
	viper.AutomaticEnv()

	// Bind specific environment variables to configuration fields.
	if err := bindEnvironmentVariables(); err != nil {
		return nil, err
	}
	// Read the configuration file.
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %s", err)
	}

	// Setup a watch on the configuration file to detect changes.
	watchConfig()

	var config Config

	// Unmarshal the configuration into the Config struct.
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %s", err)
	}

	// Validate that required configuration fields are set.
	if config.WebhookSecret == "" {
		return nil, fmt.Errorf("missing webhook secret in the configuration")
	}

	if config.GitHubToken == "" {
		return nil, fmt.Errorf("missing GitHub token in the configuration")
	}

	if config.RollbarToken == "" {
		return nil, fmt.Errorf("missing Rollbar token in the configuration")
	}
	// Resolve the local repository path.
	localRepoDir, err := getLocalRepoPath(config.Github.LocalRepo)
	if err != nil {
		return nil, err
	}
	// Update the local repository path in the configuration.
	config.Github.LocalRepo = localRepoDir

	return &config, nil
}

// bindEnvironmentVariables binds environment variables to specific configuration fields.
// This allows the application to override config file settings with environment variables.
func bindEnvironmentVariables() error {
	if err := viper.BindEnv("RollbarToken", "ROLLBAR_TOKEN"); err != nil {
		return fmt.Errorf("error binding ROLLBAR_TOKEN: %w", err)
	}
	if err := viper.BindEnv("GitHubToken", "GITHUB_TOKEN"); err != nil {
		return fmt.Errorf("error binding GITHUB_TOKEN: %w", err)
	}
	if err := viper.BindEnv("WebhookSecret", "WEBHOOK_SECRET"); err != nil {
		return fmt.Errorf("error binding WEBHOOK_SECRET: %w", err)
	}
	if err := viper.BindEnv("KubeConfig", "KUBE_CONFIG"); err != nil {
		return fmt.Errorf("error binding KUBE_CONFIG: %w", err)
	}
	return nil
}

// watchConfig sets up a watcher on the configuration file using fsnotify to detect changes.
// When a change is detected, the configuration is reloaded.
func watchConfig() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Infof("Config file %s changed:", e.Name)
		// Reload or re-unmarshal the config as necessary
		var config Config
		if err := viper.Unmarshal(&config); err != nil {
			log.Warnf("Failed to unmarshal config on reload: %s", err)
		}
		log.Info("Config reloaded successfully.")
	})
}

// getLocalRepoPath resolves the local repository path by joining the user's home directory
// with the specified localRepoSrcFolder.
func getLocalRepoPath(localRepoSrcFolder string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	log.Infof("Home directory: %s", homeDir)
	return filepath.Join(homeDir, localRepoSrcFolder), nil
}
