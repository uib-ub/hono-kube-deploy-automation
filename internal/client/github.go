package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/google/go-github/v63/github"
	log "github.com/sirupsen/logrus"
)

// GithubClient wraps the github.Client and adds custom methods.
type GithubClient struct {
	*github.Client // Embedding the github.Client struct
}

// NewGithubClient returns a new GithubClient instance with the optional authentication credentials
func NewGithubClient(githubToken string) *GithubClient {
	httpClient := &http.Client{
		Timeout: time.Second * 30,
	}
	client := github.NewClient(httpClient)
	if githubToken != "" {
		client = client.WithAuthToken(githubToken)
	}
	return &GithubClient{Client: client}
}

// GetWebhookEvent validates and parses a GitHub webhook event.
func (g *GithubClient) GetWebhookEvent(req *http.Request, WebhookSecret string) (any, error) {
	payload, err := github.ValidatePayload(req, []byte(WebhookSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to validate payload: %w", err)
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}
	log.Infof("Received webhook event type: %v\n", reflect.TypeOf(event))

	return event, nil
}

// GetPullRequest retrieves a pull request by owner, repo, and issue number.
func (g *GithubClient) GetPullRequest(
	ctx context.Context,
	owner,
	repo string,
	issueNum int,
) (*github.PullRequest, error) {
	pr, _, err := g.PullRequests.Get(ctx, owner, repo, issueNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}
	return pr, nil
}

// DeletePackageImage deletes a specific version of a package image by tag on Github.
func (g *GithubClient) DeletePackageImage(
	ctx context.Context,
	owner,
	packageType,
	packageName,
	tag string,
) error {
	encodedPackageName := url.PathEscape(packageName)
	opts := &github.PackageListOptions{PackageType: &packageType}
	// Search for version ID of the package based on tag
	packageVersions, _, err := g.Client.Users.PackageGetAllVersions(
		ctx,
		owner,
		packageType,
		encodedPackageName,
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to get package versions: %w", err)
	}
	for _, pv := range packageVersions {
		for _, t := range pv.GetMetadata().GetContainer().Tags {
			if t == tag {
				// Delete the package version based on the tag
				_, err := g.Client.Users.PackageDeleteVersion(
					ctx,
					owner,
					packageType,
					encodedPackageName,
					pv.GetID(),
				)
				if err != nil {
					return fmt.Errorf("failed to delete package version: %w", err)
				}
				log.Infof("Package %s with version tag %s is deleted!", encodedPackageName, t)
				return nil
			}
		}
	}
	return fmt.Errorf("package %s with version tag %s not found", encodedPackageName, tag)
}

// TriggerWorkFlow triggers a GitHub Actions workflow for a repository.
func (g *GithubClient) TriggerWorkFlow(
	ctx context.Context,
	owner,
	repo,
	WFFile,
	branch string,
) error {
	log.Infof("Triggering workflow %s for repo %s on branch %s.", WFFile, repo, branch)
	// Create a new workflow dispatch event
	opts := &github.CreateWorkflowDispatchEventRequest{
		Ref: branch,
	}
	if _, err := g.Client.Actions.CreateWorkflowDispatchEventByFileName(
		ctx,
		owner,
		repo,
		WFFile,
		*opts,
	); err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}
	log.Infof("Workflow %s is triggered", WFFile)

	if err := g.waitForWorkflowCompletion(ctx, owner, repo, WFFile, branch); err != nil {
		return fmt.Errorf("failed to wait for workflow completion: %w", err)
	}
	return nil
}

// waitForWorkflowCompletion waits for the triggered workflow to complete.
func (g *GithubClient) waitForWorkflowCompletion(
	ctx context.Context,
	owner,
	repo,
	WFFile,
	branch string,
) error {
	// Set the initial interval, max interval, and max duration for polling
	initialInterval := 5 * time.Second // Set to 5 seconds.
	maxInterval := 30 * time.Second    // Set to 30 seconds to prevent long delays between polls
	maxDuration := 1 * time.Minute     // Total duration of the polling loop is capped at 1 minutes.

	startTime := time.Now()
	interval := initialInterval

	// Polling loop to check the workflow status periodically
	for {
		// Wait for the current interval before polling again
		time.Sleep(interval)
		// Fetch the latest workflow status and conclusion
		status, conclusion, err := g.getLatestWorkflowRunStatus(ctx, owner, repo, WFFile, branch)
		if err != nil {
			return fmt.Errorf("failed to get latest workflow status: %w", err)
		}

		log.Infof("Current workflow %s status: %s, conclusion: %s", WFFile, status, conclusion)

		// Handle the workflow status
		if status == "completed" {
			if err := g.handleWorkflowConclusion(WFFile, conclusion); err != nil {
				return err
			}
		} else {
			log.Infof("Workflow %s is still %s", WFFile, status)
		}
		// Check if the maximum duration has been reached.
		if time.Since(startTime) >= maxDuration {
			log.Info("Maximum duration reached. Exiting polling loop.")
			break
		}
		// Exponentially increase the interval, but don't exceed the max interval
		if interval*2 < maxInterval {
			interval *= 2
		} else {
			interval = maxInterval
		}
	}
	log.Info("Polling loop completed. Now start a final check.")
	// Final check after the loop
	return g.workflowFinalCheck(ctx, owner, repo, WFFile, branch)
}

// handleWorkflowConclusion handles the conclusion of the workflow.
func (g *GithubClient) handleWorkflowConclusion(WFFile, conclusion string) error {
	switch conclusion {
	case "success":
		log.Infof("Workflow %s completed successfully", WFFile)
		// Do not return here, let the caller decide
	case "failure":
		return fmt.Errorf("workflow %s failed with conclusion: %s", WFFile, conclusion)
	case "neutral", "cancelled", "timed_out", "action_required":
		return fmt.Errorf("workflow %s ended with conclusion: %s", WFFile, conclusion)
	default:
		return fmt.Errorf("unknown workflow conclusion: %s", conclusion)
	}
	return nil
}

// workflowFinalCheck performs a final check on the workflow status and conclusion.
func (g *GithubClient) workflowFinalCheck(
	ctx context.Context,
	owner,
	repo,
	WFFile,
	branch string,
) error {
	log.Info("Performing final check on the workflow status ...")
	status, conclusion, err := g.getLatestWorkflowRunStatus(ctx, owner, repo, WFFile, branch)
	if err != nil {
		return fmt.Errorf("failed to get final workflow status: %w", err)
	}

	log.Infof("Final workflow %s status: %s, conclusion: %s", WFFile, status, conclusion)
	// Determine the final outcome based on the status and conclusion
	if status == "completed" {
		switch conclusion {
		case "success":
			log.Infof("Final check: Workflow %s completed successfully", WFFile)
			return nil
		case "failure":
			return fmt.Errorf("final check: workflow %s failed", WFFile)
		default:
			return fmt.Errorf("final check: workflow %s ended with conclusion: %s", WFFile, conclusion)
		}
	}
	return fmt.Errorf("timed out waiting for GitHub workflow completion")
}

// getLatestWorkflowRunStatus retrieves the status and conclusion of the latest workflow run.
func (g *GithubClient) getLatestWorkflowRunStatus(
	ctx context.Context,
	owner,
	repo,
	WFFile,
	branch string,
) (string, string, error) {
	// Get the latest workflow run for the workflow ID
	runs, _, err := g.Client.Actions.ListWorkflowRunsByFileName(
		ctx,
		owner,
		repo,
		WFFile,
		&github.ListWorkflowRunsOptions{Branch: branch, ListOptions: github.ListOptions{PerPage: 1}},
	)
	if err != nil {
		return "", "", err
	}
	if len(runs.WorkflowRuns) == 0 {
		return "", "", fmt.Errorf("no workflow runs found")
	}
	return runs.WorkflowRuns[0].GetStatus(), runs.WorkflowRuns[0].GetConclusion(), nil
}

// DownloadGithubRepository clones or pulls a GitHub repository to a local path.
func (g *GithubClient) DownloadGithubRepository(
	localRepoPath,
	repoFullName,
	branchName string,
) error {
	if branchName == "" {
		branchName = "main" // Default to master if no branch is specified
	}

	log.Infof("Github repository full name: %s", repoFullName)
	githubRepoUrl := fmt.Sprintf("https://github.com/%s.git", repoFullName)

	// Check if the local source directory exists
	if _, err := os.Stat(localRepoPath); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(localRepoPath, 0755); err != nil {
			return fmt.Errorf("failed to create local source directory: %w", err)
		}
	}

	if _, err := os.Stat(filepath.Join(localRepoPath, ".git")); os.IsNotExist(err) {
		// clone the repository .git doesn't exist
		log.Infof("Cloning repository %s into %s", githubRepoUrl, localRepoPath)
		if err := g.runCmd(
			"git",
			"clone",
			"-b",
			branchName,
			githubRepoUrl,
			localRepoPath,
		); err != nil {
			return fmt.Errorf("failed to clone git repository to local source path: %w", err)
		}
	} else {
		// If .git exists, pull the latest changes
		log.Infof("Pull repository %s to %s", githubRepoUrl, localRepoPath)
		if err := g.runCmd("git", "-C", localRepoPath, "pull"); err != nil {
			return fmt.Errorf("failed to pull git repository updates: %w", err)
		}
	}
	return nil
}

// runCmd runs a shell command with arguments.
func (g *GithubClient) runCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s failed: %w", command, err)
	}
	return nil
}

// DeleteLocalRepository deletes the local repository directory if it exists.
func (g *GithubClient) DeleteLocalRepository(localRepoPath string) error {
	// Remove the existing local source directory if it exists
	if _, err := os.Stat(localRepoPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(localRepoPath); err != nil {
			return fmt.Errorf("failed to delete local repository directory: %w", err)
		}
	}
	log.Infof("Local repository directory %s is removed", localRepoPath)
	return nil
}
