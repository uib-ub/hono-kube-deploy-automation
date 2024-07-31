package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/google/go-github/v63/github"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/client"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/errors"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	WebhookSecret      string
	KubeResourcePath   string
	WorkFlowFilePrefix string
}
type Server struct {
	GithubClient *client.GithubClient
	KubeClient   *client.KubeClient
	DockerClient *client.DockerClient
	Options      *Options
}

type webhookEventData struct {
	ctx                    context.Context
	namespace              string
	githubLoginOwner       string
	githubRepoFullName     string
	githubRepoName         string
	githubRepoIssueNumber  int
	githubRepoBranch       string
	githubWorkFlowFileName string
	imageTag               string
}

func NewServer(githubClient *client.GithubClient, kubeClient *client.KubeClient, dockerClient *client.DockerClient, options *Options) *Server {
	return &Server{
		GithubClient: githubClient,
		KubeClient:   kubeClient,
		DockerClient: dockerClient,
		Options:      options,
	}
}

func (s *Server) WebhookHandler(w http.ResponseWriter, req *http.Request) {
	// Parse and validate the webhook payload
	event, err := s.GithubClient.GetWebhookEvent(req, s.Options.WebhookSecret)
	if err != nil {
		log.Errorf("Get webhook event failed: %v", err)
		s.handleError(w, errors.NewInternalServerError(fmt.Sprintf("%v", err)))
		return
	}
	// Respond immediately to GitHub to avoid timeout
	fmt.Fprintf(w, "Webhook event received and being processed!")
	w.WriteHeader(http.StatusOK)

	// Process webhook events asynchronously
	log.Info("Start go routine to process webhook event...")
	go func(e any) {
		err := s.processWebhookEvents(e)
		if err != nil {
			log.Errorf("process webhook event failed: %v", err)
			s.handleError(w, err)
		} else {
			log.Info("Webhook processed successfully!")
		}
	}(event) // pass event to the go routine
}

func (s *Server) handleError(w http.ResponseWriter, err error) {
	statusCode, errMsg := errors.HandleHTTPError(err)
	http.Error(w, errMsg, statusCode)
	log.WithFields(log.Fields{"error": err, "status": statusCode}).Error(errMsg)
}

func (s *Server) processWebhookEvents(event any) error {
	switch e := event.(type) {
	case *github.Hook:
		log.Info("Received hook event")
	case *github.IssueCommentEvent:
		log.Info("Received issue comment event")
		return s.handleIssueCommentEvent(e)
	case *github.PullRequestEvent:
		log.Info("Received pull request event")
		return s.handlePullRequestEvent(e)
	default:
		return errors.NewInternalServerError(fmt.Sprintf("Unsupported event type: %v", reflect.TypeOf(e)))
	}
	return nil
}

func (s *Server) handleIssueCommentEvent(event *github.IssueCommentEvent) error {
	log.Infof("Issue Comment: action=%s, body=%s\n", event.GetAction(), event.GetComment().GetBody())

	return nil
}

func (s *Server) handlePullRequestEvent(event *github.PullRequestEvent) error {
	log.Infof("Issue Comment: action=%s\n", event.GetAction())

	return nil
}
