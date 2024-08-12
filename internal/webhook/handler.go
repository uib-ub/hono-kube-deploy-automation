package webhook

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/errors"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/util"
)

// WebhookHandler returns an HTTP handler function that processes GitHub webhook events.
// It validates the incoming webhook, responds immediately to GitHub,
// and then processes the event asynchronously.
func WebhookHandler(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse and validate the webhook payload using the GitHub client.
		event, err := s.GithubClient.GetWebhookEvent(req, s.Options.WebhookSecret)
		if err != nil {
			log.Errorf("Get webhook event failed: %v", err)
			handleError(w, errors.NewInternalServerError(fmt.Sprintf("%v", err)))
			return
		}
		// Respond immediately to GitHub to avoid triggering a timeout.
		fmt.Fprintf(w, "Webhook event received and being processed!")
		w.WriteHeader(http.StatusOK)

		// Process webhook events asynchronously in a new goroutine.
		log.Info("Start go routine to process webhook event...")
		go func(e any) {
			// Process the webhook event.
			err := s.processWebhookEvents(e)
			if err != nil {
				log.Errorf("process webhook event failed: %v", err)
				util.NotifyError(err)
			} else {
				log.Info("Webhook processed successfully!")
			}
		}(event) // Pass the event to the goroutine.
	}
}

// handleError handles HTTP errors by setting the appropriate status code
// and error message in the response.
func handleError(w http.ResponseWriter, err error) {
	statusCode, errMsg := errors.HandleHTTPError(err)
	http.Error(w, errMsg, statusCode)
	log.WithFields(log.Fields{"error": err, "status": statusCode}).Error(errMsg)
	util.NotifyError(err)
}
