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
		return nil, fmt.Errorf("failed to validate payload: %w", err)
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}
	log.Infof("Received webhook event type: %v\n", reflect.TypeOf(event))

	return event, nil
}

func (g *GithubClient) GetPullRequest(ctx context.Context, owner, repo string, issueNumber int) (*github.PullRequest, error) {
	pr, _, err := g.PullRequests.Get(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}
	return pr, nil
}

func (g *GithubClient) DeletePackageImage(ctx context.Context, owner, packageType, packageName, tag string) error {
	encodedPackageName := url.PathEscape(packageName)
	opts := &github.PackageListOptions{PackageType: &packageType}
	// Search for version ID of the package based on tag
	packageVersions, _, err := g.Client.Users.PackageGetAllVersions(ctx, owner, packageType, encodedPackageName, opts)
	if err != nil {
		return fmt.Errorf("failed to get package versions: %w", err)
	}
	for _, pv := range packageVersions {
		for _, t := range pv.GetMetadata().GetContainer().Tags {
			if t == tag {
				// Delete the package version based on the tag
				_, err := g.Client.Users.PackageDeleteVersion(ctx, owner, packageType, encodedPackageName, pv.GetID())
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

func (g *GithubClient) DownloadGithubRepository(localRepoSrcPath, repoFullName, branchName string) error {
	if branchName == "" {
		branchName = "main" // Default to master if no branch is specified
	}

	log.Infof("Github repository full name: %s", repoFullName)
	githubRepoUrl := fmt.Sprintf("https://github.com/%s.git", repoFullName)

	// Check if the local source directory exists
	if _, err := os.Stat(localRepoSrcPath); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(localRepoSrcPath, 0755); err != nil {
			return fmt.Errorf("failed to create local source directory: %w", err)
		}
	}

	if _, err := os.Stat(filepath.Join(localRepoSrcPath, ".git")); os.IsNotExist(err) {
		// clone the repository .git doesn't exist
		log.Infof("Cloning repository %s into %s", githubRepoUrl, localRepoSrcPath)
		if err := g.runCmd("git", "clone", "-b", branchName, githubRepoUrl, localRepoSrcPath); err != nil {
			return fmt.Errorf("failed to clone git repository to local source path: %w", err)
		}
	} else {
		// If .git exists, pull the latest changes
		log.Infof("Pull repository %s to %s", githubRepoUrl, localRepoSrcPath)
		if err := g.runCmd("git", "-C", localRepoSrcPath, "pull"); err != nil {
			return fmt.Errorf("failed to pull git repository updates: %w", err)
		}
	}
	return nil
}

func (g *GithubClient) runCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s failed: %w", command, err)
	}
	return nil
}

func (g *GithubClient) DeleteLocalRepository(localRepoSrcPath string) error {
	// Remove the existing local source directory if it exists
	if _, err := os.Stat(localRepoSrcPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(localRepoSrcPath); err != nil {
			return fmt.Errorf("failed to delete local repository directory: %w", err)
		}
	}
	log.Infof("Local repository directory %s is removed", localRepoSrcPath)
	return nil
}
