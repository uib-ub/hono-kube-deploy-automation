package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/jarcoal/httpmock"
)

var getWebhookEventTestCases = []struct {
	name          string
	githubClient  *GithubClient
	webhookSecret string
	eventType     string
	payload       interface{}
	expectedError bool
}{
	{
		name:          "Valid IssueComment Event",
		githubClient:  NewGithubClient(""),
		webhookSecret: "test-secret",
		eventType:     "issue_comment",
		payload: &github.IssueCommentEvent{
			Action: github.String("created"),
			Issue: &github.Issue{
				Number: github.Int(1),
			},
			Comment: &github.IssueComment{
				Body: github.String("Test comment"),
			},
		},
		expectedError: false,
	},
	{
		name:          "Valid PullRequest Event",
		githubClient:  NewGithubClient(""),
		webhookSecret: "test-secret",
		eventType:     "pull_request",
		payload: &github.PullRequestEvent{
			Action: github.String("opened"),
			PullRequest: &github.PullRequest{
				Number: github.Int(2),
				Title:  github.String("Test PR"),
			},
		},
		expectedError: false,
	},
	{
		name:          "Invalid Event",
		githubClient:  NewGithubClient(""),
		webhookSecret: "test-secret",
		eventType:     "invalid_event",
		payload:       struct{}{},
		expectedError: true,
	},
}

func TestGetWebhookEvent(t *testing.T) {
	for _, tc := range getWebhookEventTestCases {

		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", tc.eventType)
			req.Header.Set("X-Hub-Signature", "sha1="+createHmac(payload, []byte(tc.webhookSecret)))

			event, err := tc.githubClient.GetWebhookEvent(req, tc.webhookSecret)

			if (err != nil) != tc.expectedError {
				t.Errorf("GetWebhookEvent() error = %v, expectedError %v", err, tc.expectedError)
			}
			if !tc.expectedError && event == nil {
				t.Errorf("GetWebhookEvent() got nil event, expected non-nil")
			}
			if !tc.expectedError && reflect.TypeOf(event) != reflect.TypeOf(tc.payload) {
				t.Errorf("GetWebhookEvent() got = %T, want %T", event, tc.payload)
			}
		})
	}
}

func createHmac(payload []byte, secret []byte) string {
	mac := hmac.New(sha1.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

var getPullRequestTestCases = []struct {
	name          string
	githubClient  *GithubClient
	owner         string
	repo          string
	issueNum      int
	mockResponse  *github.PullRequest
	expectedError bool
}{
	{
		name:         "Valid Pull Request with test branch",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		repo:         "testrepo",
		issueNum:     1,
		mockResponse: &github.PullRequest{
			Number: github.Int(1),
			Title:  github.String("Test PR"),
			Head: &github.PullRequestBranch{
				Ref: github.String("test-branch"),
				SHA: github.String("6dcb09b"),
			},
		},
		expectedError: false,
	},
	{
		name:          "Invalid Pull Request",
		githubClient:  NewGithubClient(""),
		owner:         "testowner",
		repo:          "testrepo",
		issueNum:      999,
		mockResponse:  nil,
		expectedError: true,
	},
}

func TestGetPullRequest(t *testing.T) {
	ctx := context.Background()
	// Mock the GitHub API response
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, tc := range getPullRequestTestCases {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", tc.owner, tc.repo, tc.issueNum)

			if tc.mockResponse != nil {
				httpmock.RegisterResponder("GET", url,
					httpmock.NewJsonResponderOrPanic(200, tc.mockResponse))
			} else {
				httpmock.RegisterResponder("GET", url,
					httpmock.NewStringResponder(404, "Not found"))
			}

			pr, err := tc.githubClient.GetPullRequest(ctx, tc.owner, tc.repo, tc.issueNum)

			if (err != nil) != tc.expectedError {
				t.Errorf("GetPullRequest() error = %v, expectedError %v", err, tc.expectedError)
			}

			if !tc.expectedError && !reflect.DeepEqual(pr, tc.mockResponse) {
				t.Errorf("GetPullRequest() got = %v, want %v", pr, tc.mockResponse)
			}

			if err == nil && !tc.expectedError {
				if pr.GetHead().GetRef() != tc.mockResponse.GetHead().GetRef() {
					t.Errorf("GetPullRequest() got = %v, want %v", pr.GetHead().GetRef(), tc.mockResponse.GetHead().GetRef())
				}
				if pr.GetHead().GetSHA() != tc.mockResponse.GetHead().GetSHA() {
					t.Errorf("GetPullRequest() got = %v, want %v", pr.GetHead().GetSHA(), tc.mockResponse.GetHead().GetSHA())
				}
			}
		})
	}
}

// Test cases for testing DeletePackageImage()
var delateImageTestCases = []struct {
	ctx          context.Context
	owner        string
	packageName  string
	packageType  string
	tag          string
	githubClient *GithubClient
}{
	{
		ctx:          context.Background(),
		owner:        "uib-ub",
		packageName:  "uib-ub/uib-ub-monorepo-api",
		packageType:  "container",
		tag:          "test",
		githubClient: NewGithubClient(os.Getenv("GITHUB_TOKEN")),
	},
}

func TestDeletePackageImage(t *testing.T) {
	for i, tc := range delateImageTestCases {
		err := tc.githubClient.DeletePackageImage(
			tc.ctx,
			tc.owner,
			tc.packageType,
			tc.packageName,
			tc.tag,
		)
		if err != nil {
			t.Errorf("failed delete package image in test case %d: expected nil, got %v", i, err)
		}
	}
}

// Test cases for testing clone/pull/delete Github repositories
var githubRepositoryTestCases = []struct {
	destPath     string
	repo         string
	branch       string
	githubClient *GithubClient
}{
	{
		destPath:     os.Getenv("LOCAL_REPO_SRC"),
		repo:         "uib-ub/uib-ub-monorepo",
		branch:       "main",
		githubClient: NewGithubClient(os.Getenv("GITHUB_TOKEN")),
	},
}

func TestDownloadGithubRepository(t *testing.T) {
	for i, tc := range githubRepositoryTestCases {
		err := tc.githubClient.DownloadGithubRepository(tc.destPath, tc.repo, tc.branch)
		if err != nil {
			t.Errorf("failed to download Github repo in test case %d: expected nil, got %v", i, err)
		}
	}
}

func TestDeleteLocalRepository(t *testing.T) {
	for i, tc := range githubRepositoryTestCases {
		err := tc.githubClient.DeleteLocalRepository(tc.destPath)
		if err != nil {
			t.Errorf("failed to delete local repo in test case %d: expected nil, got %v", i, err)
		}
	}
}

var workflowTestCases = []struct {
	ctx          context.Context
	owner        string
	repo         string
	WFFile       string
	branch       string
	githubClient *GithubClient
}{
	{
		ctx:          context.Background(),
		owner:        "uib-ub",
		repo:         "uib-ub-monorepo",
		WFFile:       "deploy-kube-secrets-hono-api-test.yaml",
		branch:       "test-webhook",
		githubClient: NewGithubClient(os.Getenv("GITHUB_TOKEN")),
	},
}

func TestTriggerWorkFlow(t *testing.T) {
	for i, tc := range workflowTestCases {
		err := tc.githubClient.TriggerWorkFlow(tc.ctx, tc.owner, tc.repo, tc.WFFile, tc.branch)
		if err != nil {
			t.Errorf("failed to trigger workflow in test case %d: expected nil, got %v", i, err)
		}
	}
}
