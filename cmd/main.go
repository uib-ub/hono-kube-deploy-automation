package main

import (
	"net/http"
	"sync/atomic"

	"github.com/rollbar/rollbar-go"
	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/client"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/config"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/util"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/webhook"
)

// Declare isReady as a global variable to track readiness status
var isReady atomic.Value

func init() {
	// Initialize the log formatter to include a full timestamp in the logs.
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	// Set log level
	// log.SetLevel(log.DebugLevel)

	// Initialize readiness status
	isReady.Store(false)
}

func main() {
	// Load configuration settings from file and environment variables.
	cfg, err := config.NewConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
		return
	}
	// Log the loaded configuration for debugging purposes.
	log.WithFields(log.Fields{
		//	"RollBarToken":   cfg.RollbarToken,
		//	"GitHubToken":    cfg.GitHubToken,
		//	"WebhookSecret":  cfg.WebhookSecret,
		"KubeConfig":     cfg.KubeConfig,
		"LocalRepo":      cfg.Github.LocalRepo,
		"WorkflowPrefix": cfg.Github.WorkflowPrefix,
		"PackageType":    cfg.Github.PackageType,
		"PrDeployLabel":  cfg.Github.PrDeployLabel,
		"Resource":       cfg.Kubernetes.Resource,
		"DevNamespace":   cfg.Kubernetes.DevNamespace,
		"TestNamespace":  cfg.Kubernetes.TestNamespace,
		"Registry":       cfg.Container.Registry,
		"Dockerfile":     cfg.Container.Dockerfile,
		"ImageSuffix":    cfg.Container.ImageSuffix,
	}).Info("Configuration loaded:")

	// Set up Rollbar for monitoring errors and logging.
	rollbar.SetToken(cfg.RollbarToken)
	rollbar.SetEnvironment("production")
	rollbar.SetCodeVersion("latest")
	// Ensure Rollbar is flushed and closed on exit.
	defer rollbar.Wait()
	defer rollbar.Close()

	// Initialize the GitHub client using the provided GitHub token.
	githubClient := client.NewGithubClient(cfg.GitHubToken)

	// Initialize the Kubernetes client using the provided kubeConfig path.
	kubeClient, err := client.NewKubernetesClient(cfg.KubeConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Kubernetes client")
		util.NotifyCritical(err)
	}

	// Initialize the Docker client with the specified Docker options.
	dockerClient, err := client.NewDockerClient(&client.DockerOptions{
		ContainerRegistry: cfg.Container.Registry,
		RegistryPassword:  cfg.GitHubToken, // Using GitHub token as the registry password.
		Dockerfile:        cfg.Container.Dockerfile,
	}, nil)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Docker client")
		util.NotifyCritical(err)
	}

	// Create a new webhook server instance with the initialized clients and configuration options.
	server := webhook.NewServer(githubClient, kubeClient, dockerClient, &webhook.Options{
		WebhookSecret: cfg.WebhookSecret,
		KubeResDir:    cfg.Kubernetes.Resource,
		WFPrefix:      cfg.Github.WorkflowPrefix,
		LocalRepoDir:  cfg.Github.LocalRepo,
		PackageType:   cfg.Github.PackageType,
		PrDeployLabel: cfg.Github.PrDeployLabel,
		ImageSuffix:   cfg.Container.ImageSuffix,
		DevNamespace:  cfg.Kubernetes.DevNamespace,
		TestNamespace: cfg.Kubernetes.TestNamespace,
	})
	// Set up the HTTP route handler for the webhook endpoint.
	// When the webhook is triggered, the WebhookHandler function will be invoked.
	http.HandleFunc("/webhook", webhook.WebhookHandler(server))

	// Health check endpoints
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readinessHandler)

	// Indicate readiness once initialization is complete
	isReady.Store(true)

	// Start the HTTP server on port 8080 and log any fatal errors.
	log.Info("Server instance created, listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.WithError(err).Fatal("Failed to start server!")
		util.NotifyCritical(err)
	}
}

// healthHandler checks if the application is alive
func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Always return HTTP 200 OK to indicate the application is alive
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		log.Infof("Failed to write response: %v", err)
	}
}

// readinessHandler checks if the application is ready to handle requests
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if isReady.Load().(bool) {
		// Application is ready
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ready")); err != nil {
			log.Infof("Failed to write response: %v", err)
		}
	} else {
		// Application is not ready
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("not ready")); err != nil {
			log.Infof("Failed to write response: %v", err)
		}
	}
}
