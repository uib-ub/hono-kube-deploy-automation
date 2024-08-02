package main

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/client"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/config"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/webhook"
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
		"GitHubToken":   cfg.GitHubToken,
		"WebhookSecret": cfg.WebhookSecret,
		"LocalRepoPath": cfg.LocalRepoPath,
		"DockerFile":    cfg.DockerFile,
		"Registry":      cfg.Registry,
		"KubeResSrc":    cfg.KubeResSrc,
		"WFPrefix":      cfg.WFPrefix,
		"ImageSuffix":   cfg.ImageSuffix,
	}).Info("Configuration loaded:")

	// Get the GitHub client
	githubClient := client.NewGithubClient(cfg.GitHubToken)

	// Get kubernetes client
	kubeClient, err := client.NewKubernetesClient(cfg.KubeConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Kubernetes client")
	}

	// Get docker client
	dockerClient, err := client.NewDockerClient(&client.DockerOptions{
		ContainerRegistry: cfg.Registry,
		RegistryPassword:  cfg.GitHubToken,
		Dockerfile:        cfg.DockerFile,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Docker client")
	}
	server := webhook.NewServer(githubClient, kubeClient, dockerClient, &webhook.Options{
		WebhookSecret: cfg.WebhookSecret,
		KubeResPath:   cfg.KubeResSrc,
		WFPrefix:      cfg.WFPrefix,
		LocalRepoPath: cfg.LocalRepoPath,
		ImageSuffix:   cfg.ImageSuffix,
	})
	// Set up route handler, if webhook is triggered, then the http function will be invoked
	http.HandleFunc("/webhook", webhook.WebhookHandler(server))

	log.Info("Server instance created, listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.WithError(err).Fatal("Failed to start server!")
	}
}
