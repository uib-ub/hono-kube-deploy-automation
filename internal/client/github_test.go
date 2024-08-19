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
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/jarcoal/httpmock"
)

// Test cases for testing GetWebhookEvent
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

// Test cases for testing GetPullRequest
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

// Test cases for testing DeletePackageImage
var deleteImageTestCases = []struct {
	name          string
	githubClient  *GithubClient
	owner         string
	packageName   string
	packageType   string
	tag           string
	mockVersions  []*github.PackageVersion
	expectedError bool
}{
	{
		name:         "Valid Package Deletion",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		packageName:  "testpackage",
		packageType:  "container",
		tag:          "v1.0.0",
		mockVersions: []*github.PackageVersion{
			{
				ID: github.Int64(1),
				Metadata: &github.PackageMetadata{
					Container: &github.PackageContainerMetadata{
						Tags: []string{"v1.0.0"},
					},
				},
			},
		},
		expectedError: false,
	},
	{
		name:          "Package Not Found",
		githubClient:  NewGithubClient(""),
		owner:         "testowner",
		packageName:   "nonexistent",
		packageType:   "container",
		tag:           "v1.0.0",
		mockVersions:  []*github.PackageVersion{},
		expectedError: true,
	},
}

func TestDeletePackageImage(t *testing.T) {
	ctx := context.Background()
	// Mock the GitHub API response
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, tc := range deleteImageTestCases {
		t.Run(tc.name, func(t *testing.T) {
			listUrl := fmt.Sprintf("https://api.github.com/users/%s/packages/%s/%s/versions",
				tc.owner, tc.packageType, url.PathEscape(tc.packageName))
			httpmock.RegisterResponder("GET", listUrl,
				httpmock.NewJsonResponderOrPanic(200, tc.mockVersions))

			if len(tc.mockVersions) > 0 {
				deleteUrl := fmt.Sprintf("https://api.github.com/users/%s/packages/%s/%s/versions/%d",
					tc.owner, tc.packageType, url.PathEscape(tc.packageName), tc.mockVersions[0].GetID())
				httpmock.RegisterResponder("DELETE", deleteUrl,
					httpmock.NewStringResponder(204, ""))
			}

			err := tc.githubClient.DeletePackageImage(ctx, tc.owner, tc.packageType, tc.packageName, tc.tag)
			if (err != nil) != tc.expectedError {
				t.Errorf("DeletePackageImage() error = %v, expectedError %v", err, tc.expectedError)
			}
		})
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

func TestGithubLocalRepoLifecycle(t *testing.T) {
	for i, tc := range githubRepositoryTestCases {
		// test for cloning a repository
		t.Run("DownloadGithubRepository", func(t *testing.T) {
			err := tc.githubClient.DownloadGithubRepository(tc.destPath, tc.repo, tc.branch)
			if err != nil {
				t.Errorf("DownloadGithubRepository() error in test case %d: expected nil, got %v", i, err)
			}
		})
		// test for pulling a repository
		t.Run("DownloadGithubRepository", func(t *testing.T) {
			err := tc.githubClient.DownloadGithubRepository(tc.destPath, tc.repo, tc.branch)
			if err != nil {
				t.Errorf("DownloadGithubRepository() error in test case %d: expected nil, got %v", i, err)
			}
		})
		// test for deleting a repository
		t.Run("DeleteLocalRepository", func(t *testing.T) {
			err := tc.githubClient.DeleteLocalRepository(tc.destPath)
			if err != nil {
				t.Errorf("DeleteLocalRepository() error in test case %d: expected nil, got %v", i, err)
			}
			// Check if the directory was actually deleted
			if _, err := os.Stat(tc.destPath); !os.IsNotExist(err) {
				t.Errorf("DeleteLocalRepository() failed to delete the directory")
			}
		})
	}
}

// Test cases for testing triggering GitHub workflows
var workflowTestCases = []struct {
	name          string
	githubClient  *GithubClient
	owner         string
	repo          string
	wfFile        string
	branch        string
	mockResponses map[string]httpmock.Responder
	expectedError bool
}{
	{
		name:         "Successful Workflow Trigger",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		repo:         "testrepo",
		wfFile:       "test.yml",
		branch:       "main",
		mockResponses: map[string]httpmock.Responder{
			"POST /repos/testowner/testrepo/actions/workflows/test.yml/dispatches": httpmock.NewStringResponder(204, ""),
			"GET /repos/testowner/testrepo/actions/workflows/test.yml/runs": httpmock.NewJsonResponderOrPanic(200, github.WorkflowRuns{
				WorkflowRuns: []*github.WorkflowRun{
					{Status: github.String("completed"), Conclusion: github.String("success")},
				},
			}),
		},
		expectedError: false,
	},
	{
		name:         "Workflow Fails with Conclusion: failure",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		repo:         "testrepo",
		wfFile:       "test.yml",
		branch:       "main",
		mockResponses: map[string]httpmock.Responder{
			"POST /repos/testowner/testrepo/actions/workflows/test.yml/dispatches": httpmock.NewStringResponder(204, ""),
			"GET /repos/testowner/testrepo/actions/workflows/test.yml/runs": httpmock.NewJsonResponderOrPanic(200, github.WorkflowRuns{
				WorkflowRuns: []*github.WorkflowRun{
					{Status: github.String("completed"), Conclusion: github.String("failure")},
				},
			}),
		},
		expectedError: true,
	},
	{
		name:         "Workflow Timeout",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		repo:         "testrepo",
		wfFile:       "test.yml",
		branch:       "main",
		mockResponses: map[string]httpmock.Responder{
			"POST /repos/testowner/testrepo/actions/workflows/test.yml/dispatches": httpmock.NewStringResponder(204, ""),
			"GET /repos/testowner/testrepo/actions/workflows/test.yml/runs": httpmock.NewJsonResponderOrPanic(200, github.WorkflowRuns{
				WorkflowRuns: []*github.WorkflowRun{
					{Status: github.String("in_progress"), Conclusion: github.String("")},
				},
			}),
		},
		expectedError: true,
	},
	{
		name:         "Workflow Status Unavailable",
		githubClient: NewGithubClient(""),
		owner:        "testowner",
		repo:         "testrepo",
		wfFile:       "test.yml",
		branch:       "main",
		mockResponses: map[string]httpmock.Responder{
			"POST /repos/testowner/testrepo/actions/workflows/test.yml/dispatches": httpmock.NewStringResponder(204, ""),
			"GET /repos/testowner/testrepo/actions/workflows/test.yml/runs":        httpmock.NewStringResponder(404, "Not found"),
		},
		expectedError: true,
	},
}

func TestTriggerWorkFlow(t *testing.T) {
	ctx := context.Background()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, tc := range workflowTestCases {
		t.Run(tc.name, func(t *testing.T) {
			for url, responder := range tc.mockResponses {
				httpmock.RegisterResponder(strings.Split(url, " ")[0], "https://api.github.com"+strings.Split(url, " ")[1], responder)
			}

			err := tc.githubClient.TriggerWorkFlow(ctx, tc.owner, tc.repo, tc.wfFile, tc.branch)

			if (err != nil) != tc.expectedError {
				t.Errorf("TriggerWorkFlow() error = %v, expectedError %v", err, tc.expectedError)
			}
		})
	}
}

// Test cases for testing handleWorkflowConclusion method
var handleWorkflowConclusionTestCases = []struct {
	name         string
	githubClient *GithubClient
	wfFile       string
	conclusion   string
}{
	{
		name:         "Workflow Success",
		githubClient: NewGithubClient(""),
		wfFile:       "test.yml",
		conclusion:   "success",
	},
	{
		name:         "Workflow Failure",
		githubClient: NewGithubClient(""),
		wfFile:       "test.yml",
		conclusion:   "failure",
	},
	{
		name:         "Workflow cancelled",
		githubClient: NewGithubClient(""),
		wfFile:       "test.yml",
		conclusion:   "cancelled",
	},
	{
		name:         "Workflow timed out",
		githubClient: NewGithubClient(""),
		wfFile:       "test.yml",
		conclusion:   "timed_out",
	},
	{
		name:         "Workflow Unknown Conclusion",
		githubClient: NewGithubClient(""),
		wfFile:       "test.yml",
		conclusion:   "unknown",
	},
}

func TestHandleWorkflowConclusion(t *testing.T) {
	for _, tc := range handleWorkflowConclusionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.githubClient.handleWorkflowConclusion(tc.wfFile, tc.conclusion)

			if tc.conclusion == "success" && err != nil {
				t.Errorf("handleWorkflowConclusion() error = %v, expected nil", err)
			}
			if tc.conclusion != "success" && err == nil {
				t.Errorf("handleWorkflowConclusion() error = nil, expected non-nil")
			}
		})
	}
}
