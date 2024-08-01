package webhook

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/errors"
)

func WebhookHandler(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse and validate the webhook payload
		event, err := s.GithubClient.GetWebhookEvent(req, s.Options.WebhookSecret)
		if err != nil {
			log.Errorf("Get webhook event failed: %v", err)
			handleError(w, errors.NewInternalServerError(fmt.Sprintf("%v", err)))
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
				handleError(w, err)
			} else {
				log.Info("Webhook processed successfully!")
			}
		}(event) // pass event to the go routine
	}
}

func handleError(w http.ResponseWriter, err error) {
	statusCode, errMsg := errors.HandleHTTPError(err)
	http.Error(w, errMsg, statusCode)
	log.WithFields(log.Fields{"error": err, "status": statusCode}).Error(errMsg)
}
