package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	GitHubToken   string
	WebhookSecret string
	KubeConfig    string
	Github        GithubConfig
	Kubernetes    KubernetesConfig
	Container     ContainerConfig
}

type GithubConfig struct {
	WorkflowPrefix string
	LocalRepo      string
}

type KubernetesConfig struct {
	Resource      string
	DevNamespace  string
	TestNamespace string
}

type ContainerConfig struct {
	Registry    string
	Dockerfile  string
	ImageSuffix string
}

const (
	configPath = "./internal/config"
	configName = "config"
	configType = "yaml"
)

func NewConfig() (*Config, error) {
	// Set the base properties of Viper
	viper.SetConfigType(configType)
	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	// Automatically use environment variables where available
	viper.AutomaticEnv()

	// Bind specific environment variables
	if err := bindEnvironmentVariables(); err != nil {
		return nil, err
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %s", err)
	}

	// Setup watch on configuration file changes
	watchConfig()

	var config Config

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %s", err)
	}

	if config.WebhookSecret == "" || config.GitHubToken == "" {
		return nil, fmt.Errorf("missing required configuration")
	}

	localRepoDir, err := getLocalRepoPath(config.Github.LocalRepo)
	if err != nil {
		return nil, err
	}
	// update the local repo path
	config.Github.LocalRepo = localRepoDir

	return &config, nil
}

func bindEnvironmentVariables() error {
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

func getLocalRepoPath(localRepoSrcFolder string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	log.Infof("Home directory: %s", homeDir)
	return filepath.Join(homeDir, localRepoSrcFolder), nil
}
