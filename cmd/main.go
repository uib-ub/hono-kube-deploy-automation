package main

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/config"
	apiserver "github.com/uib-ub/hono-kube-deploy-automation/internal/api"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/client"
)

func init() {
	// Log uses timestamp
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	log.SetLevel(log.DebugLevel)
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	log.WithFields(log.Fields{
		"GitHubToken":        cfg.GitHubToken,
		"WebhookSecret":      cfg.WebhookSecret,
		"LocalRepoSrcPath":   cfg.LocalRepoSrcPath,
		"DockerFile":         cfg.DockerFile,
		"ContainerRegistry":  cfg.ContainerRegistry,
		"KubeResourcePath":   cfg.KubeResourcePath,
		"WorkFlowFilePrefix": cfg.WorkFlowFilePrefix,
	}).Info("Configuration loaded:")

	// Get the GitHub client
	githubClient := client.NewGithubClient(cfg.GitHubToken)

	// Get kubernetes client
	kubeClient, err := client.NewKubernetesClient(cfg.KubeConfigFile)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Kubernetes client")
	}

	// Get docker client
	dockerClient, err := client.NewDockerClient(&client.DockerOptions{
		ContainerRegistry:         cfg.ContainerRegistry,
		ContainerRegistryPassword: cfg.GitHubToken,
		Dockerfile:                cfg.DockerFile,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Docker client")
	}

	serverInstance := apiserver.NewServer(githubClient, kubeClient, dockerClient, &apiserver.Options{
		WebhookSecret:      cfg.WebhookSecret,
		KubeResourcePath:   cfg.KubeResourcePath,
		WorkFlowFilePrefix: cfg.WorkFlowFilePrefix,
		LocalRepoSrcPath:   cfg.LocalRepoSrcPath,
	})
	// Set up route handler, if webhook is triggered, then the http function will be invoked
	http.HandleFunc("/webhook", serverInstance.WebhookHandler)

	log.Info("Server instance created, listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.WithError(err).Fatal("Failed to start server!")
	}
}
