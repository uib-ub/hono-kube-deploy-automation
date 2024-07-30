package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/config"
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

}
