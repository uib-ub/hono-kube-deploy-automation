package api

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/google/go-github/v63/github"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/errors"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	WebhookSecret string
}

func NewServer(webhookSecret string) *Server {
	return &Server{
		WebhookSecret: webhookSecret,
	}
}

func (s *Server) WebhookHandler(w http.ResponseWriter, req *http.Request) {
	// Parse and validate the webhook payload
	event, err := s.getWebhookEvent(req)
	if err != nil {
		log.Errorf("Get webhook event failed: %v", err)
		s.handleError(w, err)
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

func (s *Server) getWebhookEvent(req *http.Request) (any, error) {
	payload, err := github.ValidatePayload(req, []byte(s.WebhookSecret))
	if err != nil {
		return nil, errors.NewInternalServerError(fmt.Sprintf("Validate payload failed: %v", err))
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return nil, errors.NewInternalServerError(fmt.Sprintf("Parse webhook failed: %v", err))
	}
	log.Infof("Received event type: %v\n", reflect.TypeOf(event))

	return event, nil
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
