package webhook

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/client"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/errors"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/util"

	log "github.com/sirupsen/logrus"
)

// Options holds the configuration options for the webhook server.
type Options struct {
	WebhookSecret string // Webhook Secret key.
	KubeResDir    string // Path to the Kubernetes resource directory.
	WFPrefix      string // Prefix used for workflow files.
	LocalRepoDir  string // Path to the local Git repository.
	ImageSuffix   string // Suffix to append to container image names.
	DevNamespace  string // Namespace for the dev environment on kubernetes.
	TestNamespace string // Namespace for the test environment on kubernetes.
}

// Server encapsulates the clients and options needed to handle webhook events,
// manage containerization, and handle Kubernetes deployment.
type Server struct {
	GithubClient *client.GithubClient // GitHub client for interacting with the GitHub API.
	KubeClient   *client.KubeClient   // Kubernetes client for managing Kubernetes resources.
	DockerClient *client.DockerClient // Docker client for managing containerization.
	Options      *Options             // Configuration options for the server.
}

// eventData contains information extracted from a webhook event that is used for processing.
type eventData struct {
	ctx            context.Context // Context for managing request lifetime.
	namespace      string          // Target namespace in Kubernetes.
	ghLoginOwner   string          // GitHub login owner.
	ghRepoFullName string          // Full name of GitHub repository.
	ghRepoName     string          // Name of the repository.
	ghIssueNum     int             // GitHub repository pull request issue number.
	ghBranch       string          // GitHub repository branch.
	ghWorkFlowFile string          // GitHub workflow file name.
	imageTag       string          // Image tag for containerization.
	imageName      string          // Image name for containerization.
}

// NewServer creates a new Server instance with the provided clients and options.
func NewServer(
	githubClient *client.GithubClient,
	kubeClient *client.KubeClient,
	dockerClient *client.DockerClient,
	options *Options,
) *Server {
	return &Server{
		GithubClient: githubClient,
		KubeClient:   kubeClient,
		DockerClient: dockerClient,
		Options:      options,
	}
}

// processWebhookEvents processes two types of GitHub webhook events, including
// issue commnet events and pull request events.
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
		errMsg := fmt.Sprintf("Unsupported event type: %v", reflect.TypeOf(e))
		return errors.NewInternalServerError(errMsg)
	}
	return nil
}

// handleIssueCommentEvent processes a GitHub issue comment event, particularly for "deploy dev" comments.
func (s *Server) handleIssueCommentEvent(event *github.IssueCommentEvent) error {
	isPullRequest := event.GetIssue().IsPullRequest()
	commentBody := event.GetComment().GetBody()
	// Check if the comment is on a pull request and contains the deploy command "deploy dev"
	if isPullRequest && strings.Contains(commentBody, "deploy dev") {
		log.Infof("Issue Comment: action=%s, comment=%s", event.GetAction(), commentBody)
		// Extract event data for processing.
		data, err := s.extractEventData(event, s.Options.DevNamespace)
		if err != nil {
			errMsg := fmt.Sprintf("failed to extract webhook event data: %v", err)
			return errors.NewInternalServerError(errMsg)
		}
		// Clone or pull the GitHub repository to the local source path.
		if err := s.getGithubRepo(data.ghRepoFullName, data.ghBranch); err != nil {
			return errors.NewInternalServerError(fmt.Sprintf("%v", err))
		}
		// Generate Kubernetes resources for the dev environment using Kustomize.
		kubeResources, err := s.handleKustomization(data.namespace)
		if err != nil {
			return errors.NewInternalServerError(fmt.Sprintf("%v", err))
		}
		// Handle the event based on the action (created/edited or deleted).
		if event.GetAction() == "deleted" {
			// Clean up the deployment/image if the comment was deleted.
			log.Info("PR comment 'deploy dev' deleted!")
			util.NotifyLog("PR comment 'deploy dev' deleted!")
			if err := s.issueCommentEventCleanup(data, &kubeResources); err != nil {
				return errors.NewInternalServerError(fmt.Sprintf("%v", err))
			}
		} else {
			// Deploy or update the resources if the comment was created or edited.
			log.Info("PR comment 'deploy dev' found!")
			util.NotifyLog("PR comment 'deploy dev' found!")
			if err := s.issueCommentEventDeploy(data, &kubeResources); err != nil {
				return errors.NewInternalServerError(fmt.Sprintf("%v", err))
			}
		}
	} else if strings.Contains(commentBody, "Vercel for Git") {
		log.Infof("No action needed for issue comment related to Vercel for Git.")
		util.NotifyLog("No action needed for issue comment related to Vercel for Git.")
	} else {
		log.Infof("No action needed for issue comment: %s", commentBody)
		util.NotifyLog("No action needed for issue comment: %s", commentBody)
	}

	return nil
}

// handlePullRequestEvent processes a GitHub pull request event,
// particularly when a pull request is merged into the main branch.
func (s *Server) handlePullRequestEvent(event *github.PullRequestEvent) error {
	baseRef := event.GetPullRequest().GetBase().GetRef()
	action := event.GetAction()
	isMerged := event.GetPullRequest().GetMerged()
	// Check if the pull request was merged to the master branch
	if baseRef == "main" && action == "closed" && isMerged {
		log.Infof("Issue Comment: action=%s\n", event.GetAction())
		// Extract event data for processing.
		data, err := s.extractEventData(event, s.Options.TestNamespace)
		if err != nil {
			return errors.NewInternalServerError(fmt.Sprintf("%v", err))
		}
		log.Infof("Pull request merged to %s branch", data.ghBranch)
		util.NotifyLog("Pull request merged to %s branch", data.ghBranch)
		// Get pull request label and check if it is "deploy-api-test"
		for _, label := range event.GetPullRequest().Labels {
			if label.GetName() == "deploy-api-test" {
				log.Infof("Pull request label: %s", label.GetName())
				// Clone or pull the GitHub repository to the local source path.
				if err := s.getGithubRepo(data.ghRepoFullName, data.ghBranch); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				// Generate Kubernetes resources for the test environment using Kustomize.
				kubeResources, err := s.handleKustomization(data.namespace)
				if err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}

				log.Info("Deploy test environment after merging!")
				// Deploy the test environment.
				if err := s.pullRequestEventDeploy(data, &kubeResources); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				// Clean up after the deployment.
				if err := s.pullRequestEventCleanup(data); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				break
			}
		}
	}
	return nil
}

// extractEventData extracts relevant data from the GitHub webhook event
// and populates the eventData structure.
func (s *Server) extractEventData(event any, namespace string) (*eventData, error) {
	ctx := context.Background()
	data := &eventData{
		ctx:            ctx,
		namespace:      namespace,
		ghWorkFlowFile: fmt.Sprintf("%s-%s.yaml", s.Options.WFPrefix, namespace),
	}
	switch event := event.(type) {
	case *github.IssueCommentEvent:
		// Extract data specific to an issue comment event.
		data.ghLoginOwner = event.GetRepo().GetOwner().GetLogin()
		data.ghRepoFullName = event.GetRepo().GetFullName()
		data.ghRepoName = event.GetRepo().GetName()
		data.ghIssueNum = event.GetIssue().GetNumber()
		// Retrieve the associated pull request.
		pr, err := s.GithubClient.GetPullRequest(
			ctx,
			data.ghLoginOwner,
			data.ghRepoName,
			data.ghIssueNum,
		)
		if err != nil {
			return nil, err
		}
		data.ghBranch = pr.GetHead().GetRef()
		data.imageTag = pr.GetHead().GetSHA()[:7] // Use the latest commit SHA as the image tag.
	case *github.PullRequestEvent:
		// Extract data specific to a pull request event.
		data.ghLoginOwner = event.GetRepo().GetOwner().GetLogin()
		data.ghRepoFullName = event.GetRepo().GetFullName()
		data.ghBranch = event.GetPullRequest().GetBase().GetRef()
		data.imageTag = "latest" // Use "latest" as the image tag.
	default:
		return nil, fmt.Errorf("unsupported event type: %v", reflect.TypeOf(event))
	}

	// Generate the container image name based on the repository full name and optional suffix.
	data.imageName = s.getImageName(data.ghRepoFullName)
	log.Debugf("Image name: %s, image tag: %s\n", data.imageName, data.imageTag)

	return data, nil
}

// getImageName generates the container image name based on the repository full name and optional suffix.
func (s *Server) getImageName(repoFullName string) string {
	if s.Options.ImageSuffix != "" {
		return fmt.Sprintf("%s-%s", repoFullName, s.Options.ImageSuffix)
	}
	return repoFullName
}

// getGithubRepo clones or pulls the GitHub repository to the local source path based on the branch name.
func (s *Server) getGithubRepo(ghRepoFullName, ghBranch string) error {
	return s.GithubClient.DownloadGithubRepository(s.Options.LocalRepoDir, ghRepoFullName, ghBranch)
}

// handleKustomization generates Kubernetes resources for the specified namespace using Kustomize.
func (s *Server) handleKustomization(ns string) ([]string, error) {
	deploykubeResPath := filepath.Join(s.Options.LocalRepoDir, s.Options.KubeResDir, ns)
	kustomizer := client.NewKustomizer(deploykubeResPath)
	return kustomizer.Build()
}

// issueCommentEventDeploy handles the deployment of resources in response to an issue comment event.
func (s *Server) issueCommentEventDeploy(data *eventData, kubeResources *[]string) error {
	// Build and push the container image.
	log.Infof("Build and push the container image for %s enviroment...", data.namespace)
	util.NotifyLog("Build and push the container image for %s enviroment...", data.namespace)
	if err := s.handleContainerization(
		"deploy",
		data.ghLoginOwner,
		data.imageName,
		data.imageTag,
	); err != nil {
		return err
	}
	log.Info("Build and push container image finished!")
	util.NotifyLog("Build and push container image finished!")
	// Deploy the resources to Kubernetes.
	log.Infof("Deploy the resources on Kubernetes for %s enviroment...", data.namespace)
	util.NotifyLog("Deploy the resources on Kubernetes for %s enviroment...", data.namespace)
	return s.deployKubeResources(data, kubeResources)
}

// issueCommentEventCleanup handles the cleanup of resources in response to an issue comment deletion.
func (s *Server) issueCommentEventCleanup(data *eventData, kubeResources *[]string) error {
	// Delete the deployment on Kubernetes.
	log.Infof("Delete the deployment on Kubernetes for %s enviroment...", data.namespace)
	util.NotifyLog("Delete the deployment on Kubernetes for %s enviroment...", data.namespace)
	if err := s.cleanupKubeResoureces(data, kubeResources); err != nil {
		return err
	}
	log.Infof("Deleting the deployment on Kubernetes for %s enviroment is finished!", data.namespace)
	util.NotifyLog("Deleting the deployment on Kubernetes for %s enviroment is finished!", data.namespace)

	// Delete the container image and repository.
	log.Infof("Delete the container image and repository for %s enviroment...", data.namespace)
	util.NotifyLog("Delete the container image and repository for %s enviroment...", data.namespace)
	if err := s.handleContainerization(
		"delete",
		data.ghLoginOwner,
		data.imageName,
		data.imageTag,
	); err != nil {
		return err
	}
	// Clean up the local source repository.
	if err := s.cleanupLocalRepository(); err != nil {
		return err
	}
	// Clean up the container image on GitHub packages.
	return s.cleanupImageOnGithub(data.ctx, data.ghLoginOwner, data.imageName, data.imageTag)
}

// pullRequestEventDeploy handles the deployment of resources in response to a pull request event.
func (s *Server) pullRequestEventDeploy(data *eventData, kubeResources *[]string) error {
	// Build and push the container image.
	log.Infof("Build and push the container image for %s enviroment...", data.namespace)
	util.NotifyLog("Build and push the container image for %s enviroment...", data.namespace)
	if err := s.handleContainerization(
		"deploy",
		data.ghLoginOwner,
		data.imageName,
		data.imageTag,
	); err != nil {
		return err
	}
	log.Info("Build and push container image finished!")
	util.NotifyLog("Build and push container image finished!")

	// Deploy the resources to Kubernetes.
	log.Infof("Deploy the resources on Kubernetes for %s enviroment...", data.namespace)
	util.NotifyLog("Deploy the resources on Kubernetes for %s enviroment...", data.namespace)
	return s.deployKubeResources(data, kubeResources)
}

// pullRequestEventCleanup handles the cleanup of resources after a pull request event.
func (s *Server) pullRequestEventCleanup(data *eventData) error {
	// Delete the container image.
	log.Infof("Delete container image for %s enviroment...", data.namespace)
	util.NotifyLog("Delete container image for %s enviroment...", data.namespace)
	if err := s.handleContainerization(
		"delete",
		data.ghLoginOwner,
		data.imageName,
		data.imageTag,
	); err != nil {
		return err
	}

	// Clean up the local source repository.
	log.Infof("Clean up the local source repository for %s enviroment...", data.namespace)
	util.NotifyLog("Clean up the local source repository for %s enviroment...", data.namespace)
	return s.cleanupLocalRepository()
}

// handleContainerization handles the build/push or deletion of container images based on the specified action.
func (s *Server) handleContainerization(action, ghLoginOwner, imageName, imageTag string) error {
	switch action {
	case "delete":
		// Delete the container image.
		return s.DockerClient.ImageDelete(ghLoginOwner, imageName, imageTag)
	case "deploy":
		// Build and push the container image.
		if err := s.DockerClient.ImageBuild(
			ghLoginOwner,
			imageName,
			imageTag,
			s.Options.LocalRepoDir,
		); err != nil {
			return err
		}
		return s.DockerClient.ImagePush(ghLoginOwner, imageName, imageTag)
	}
	return nil
}

// cleanupLocalRepository deletes the local Git repository used for the deployment.
func (s *Server) cleanupLocalRepository() error {
	return s.GithubClient.DeleteLocalRepository(s.Options.LocalRepoDir)
}

// cleanupImageOnGithub deletes the specified container image from GitHub packages.
func (s *Server) cleanupImageOnGithub(
	ctx context.Context,
	ghLoginOwner,
	imageName,
	imageTag string,
) error {
	packageType := "container"
	log.Infof("Deleting the package image %s:%s on Github...", imageName, imageTag)
	util.NotifyLog("Deleting the package image %s:%s on Github...", imageName, imageTag)
	return s.GithubClient.DeletePackageImage(ctx, ghLoginOwner, packageType, imageName, imageTag)
}

// deployKubeResources deploys Kubernetes resources extracted from the Kustomize build.
func (s *Server) deployKubeResources(data *eventData, kubeResources *[]string) error {
	// Deploy the namespace resource first.
	for _, res := range *kubeResources {
		if strings.Contains(res, "Namespace") {
			log.Debugf("found Namespace file:\n%s\n", res)
			if _, _, err := s.KubeClient.Deploy(
				data.ctx,
				[]byte(res),
				data.namespace,
			); err != nil {
				return fmt.Errorf("failed to deploy namespace: %v", err)
			}
			break
		}
	}

	// Trigger GitHub workflow to deploy Kubernetes secrets.
	if err := s.GithubClient.TriggerWorkFlow(
		data.ctx,
		data.ghLoginOwner,
		data.ghRepoName,
		data.ghWorkFlowFile,
		data.ghBranch,
	); err != nil {
		return err
	}

	// Deploy the remaining resources.
	var (
		deploymentLabels map[string]string
		expectedPods     int32
	)
	for _, res := range *kubeResources {
		if strings.Contains(res, "Namespace") {
			continue
		}
		log.Infof("data image tag: %s", data.imageTag)
		if strings.Contains(res, "kind: Deployment") && data.imageTag != "latest" {
			res = strings.Replace(res, "latest", data.imageTag, -1)
			log.Infof("replaced image tag: %s in res: %s", data.imageTag, res)
		}
		log.Debugf("Deploying resource:\n%s\n", res)
		labels, replicas, err := s.KubeClient.Deploy(data.ctx, []byte(res), data.namespace)
		if err != nil {
			return err
		}
		if strings.Contains(res, "kind: Deployment") {
			deploymentLabels = labels
			expectedPods = replicas
		}
	}
	log.Infof("Deployment labels: %v, expected pods: %d", deploymentLabels, expectedPods)
	log.Info("Deployment completed!")
	util.NotifyLog("Deployment completed!")

	// Wait for the pods to be active and running.
	if err := s.KubeClient.WaitForPodsRunning(data.ctx, data.namespace, deploymentLabels, expectedPods); err != nil {
		return err
	}
	return nil
}

// cleanupKubeResoureces deletes the Kubernetes resources extracted from the Kustomize build.
func (s *Server) cleanupKubeResoureces(data *eventData, kubeResources *[]string) error {
	for _, res := range *kubeResources {
		if strings.Contains(res, "kind: Deployment") {
			res = strings.Replace(res, "latest", data.imageTag, -1)
		}
		log.Debugf("Delete resource:\n%s\n", res)
		if err := s.KubeClient.Delete(data.ctx, []byte(res), data.namespace); err != nil {
			return err
		}
	}
	log.Info("Cleanup completed!")
	return nil
}
