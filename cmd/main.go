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
	}).Info("Configuration loaded:")

	// TODO: Get the GitHub client
	githubClient := client.NewGithubClient(cfg.GitHubToken)

	// TODO: Get kubernetes client

	// TODO: Get docker client

	serverInstance := apiserver.NewServer(githubClient, &apiserver.Options{
		WebhookSecret: cfg.WebhookSecret,
	})
	// Set up route handler, if webhook is triggered, then the http function will be invoked
	http.HandleFunc("/webhook", serverInstance.WebhookHandler)

	log.Info("Server instance created, listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.WithError(err).Fatal("Failed to start server!")
	}

}
