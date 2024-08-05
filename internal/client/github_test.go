package client

import (
	"context"
	"os"
	"testing"
)

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
