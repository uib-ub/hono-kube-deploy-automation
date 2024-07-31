package client

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/google/go-github/v63/github"
	log "github.com/sirupsen/logrus"
)

type GithubClient struct {
	*github.Client // Embedding the github.Client struct
}

func NewGithubClient(githubAccessToken string) *GithubClient {
	httpClient := &http.Client{
		Timeout: time.Second * 30,
	}
	client := github.NewClient(httpClient)
	if githubAccessToken != "" {
		client = client.WithAuthToken(githubAccessToken)
	}
	return &GithubClient{Client: client}
}

func (g *GithubClient) GetWebhookEvent(req *http.Request, webhookSecretKey string) (any, error) {
	payload, err := github.ValidatePayload(req, []byte(webhookSecretKey))
	if err != nil {
		return nil, fmt.Errorf("validate payload failed: %w", err)
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return nil, fmt.Errorf("parse webhook failed: %w", err)
	}
	log.Infof("Received webhook event type: %v\n", reflect.TypeOf(event))

	return event, nil
}

func (g *GithubClient) GetPullRequest(ctx context.Context, owner, repo string, issueNumber int) (*github.PullRequest, error) {
	pr, _, err := g.PullRequests.Get(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("get pull request failed: %w", err)
	}
	return pr, nil
}
