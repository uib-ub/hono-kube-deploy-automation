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

	log "github.com/sirupsen/logrus"
)

type Options struct {
	WebhookSecret string // Webhook Secret
	KubeResDir    string // Kube Resource Path
	WFPrefix      string // Workflow File Prefix
	LocalRepoDir  string // Local Repository Source Path
	ImageSuffix   string // Image Name Suffix
}
type Server struct {
	GithubClient *client.GithubClient
	KubeClient   *client.KubeClient
	DockerClient *client.DockerClient
	Options      *Options
}
type eventData struct {
	ctx            context.Context
	namespace      string // namespace
	ghLoginOwner   string // GitHub login owner
	ghRepoFullName string // Full name of GitHub repository
	ghRepoName     string // Name of the repository
	ghIssueNum     int    // GitHub repository issue number
	ghBranch       string // GitHub repository branch
	ghWorkFlowFile string // GitHub workflow file name
	imageTag       string // Image tag
	imageName      string // Image name
}

func NewServer(githubClient *client.GithubClient, kubeClient *client.KubeClient, dockerClient *client.DockerClient, options *Options) *Server {
	return &Server{
		GithubClient: githubClient,
		KubeClient:   kubeClient,
		DockerClient: dockerClient,
		Options:      options,
	}
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

	eventData, err := s.extractEventData(event, "dev")
	if err != nil {
		return errors.NewInternalServerError(fmt.Sprintf("failed to extract webhook event data: %v", err))
	}
	log.Infof("Webhook Event Data: %+v\n", eventData)
	// Check if the comment is on a pull request and contains the deploy command "deploy dev"
	if event.GetIssue().IsPullRequest() && strings.Contains(event.GetComment().GetBody(), "deploy dev") {
		// Get github repository to local source path
		if err := s.getGithubRepo(eventData.ghRepoFullName, eventData.ghBranch); err != nil {
			return errors.NewInternalServerError(fmt.Sprintf("%v", err))
		}
		// Get kubernetes resources by kustomization for for the dev environment
		kubeResources, err := s.handleKustomization(eventData.namespace)
		if err != nil {
			return errors.NewInternalServerError(fmt.Sprintf("%v", err))
		}
		if event.GetAction() == "deleted" {
			// Handle the delete action: clean up the deployment/image
			log.Info("PR comment 'deploy dev' deleted!")
			if err := s.issueCommentEventCleanup(eventData, &kubeResources); err != nil {
				return errors.NewInternalServerError(fmt.Sprintf("%v", err))
			}
		} else {
			// Handle the create/edit action: create/update the deployment
			log.Info("PR comment 'deploy dev' found!")
			if err := s.issueCommentEventDeploy(eventData, &kubeResources); err != nil {
				return errors.NewInternalServerError(fmt.Sprintf("%v", err))
			}
		}
	}
	return nil
}

func (s *Server) handlePullRequestEvent(event *github.PullRequestEvent) error {
	log.Infof("Issue Comment: action=%s\n", event.GetAction())
	eventData, err := s.extractEventData(event, "test")
	if err != nil {
		return errors.NewInternalServerError(fmt.Sprintf("%v", err))
	}
	// Check if the pull request was merged to the master branch
	if eventData.ghBranch == "main" && event.GetAction() == "closed" && event.GetPullRequest().GetMerged() {
		log.Infof("Pull request merged to %s branch", eventData.ghBranch)
		// Get pull request label and check if it is "deploy-api-test"
		for _, label := range event.GetPullRequest().Labels {
			if label.GetName() == "deploy-api-test" {
				log.Infof("Pull request label: %s", label.GetName())
				// Get github repository to local source path
				if err := s.getGithubRepo(eventData.ghRepoFullName, eventData.ghBranch); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				// Get kubernetes resources by kustomization for for the dev environment
				kubeResources, err := s.handleKustomization(eventData.namespace)
				if err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}

				log.Info("Deploy test environment after merging!")
				if err := s.pullRequestEventDeploy(eventData, &kubeResources); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				if err := s.pullRequestEventCleanup(eventData); err != nil {
					return errors.NewInternalServerError(fmt.Sprintf("%v", err))
				}
				break
			}
		}
	}
	return nil
}

func (s *Server) extractEventData(event any, namespace string) (*eventData, error) {
	ctx := context.Background()
	eventData := &eventData{
		ctx:            ctx,
		namespace:      namespace,
		ghWorkFlowFile: fmt.Sprintf("%s-%s.yaml", s.Options.WFPrefix, namespace),
	}
	switch event := event.(type) {
	case *github.IssueCommentEvent:
		// TODO: Debug
		log.Debugf("rep org login: %s, org name: %s, owner name: %s, owner login: %s\n",
			event.GetRepo().GetOrganization().GetLogin(),
			event.GetRepo().GetOrganization().GetName(),
			event.GetRepo().GetOwner().GetName(),
			event.GetRepo().GetOwner().GetLogin(),
		)
		eventData.ghLoginOwner = event.GetRepo().GetOwner().GetLogin()
		eventData.ghRepoFullName = event.GetRepo().GetFullName()
		eventData.ghRepoName = event.GetRepo().GetName()
		eventData.ghIssueNum = event.GetIssue().GetNumber()
		// Get pull request
		pr, err := s.GithubClient.GetPullRequest(ctx, eventData.ghLoginOwner, eventData.ghRepoName, eventData.ghIssueNum)
		if err != nil {
			return nil, err
		}
		eventData.ghBranch = pr.GetHead().GetRef()
		eventData.imageTag = pr.GetHead().GetSHA()[:7] // the latest commit SHA in a issue comment event
	case *github.PullRequestEvent:
		eventData.ghLoginOwner = event.GetRepo().GetOwner().GetLogin()
		eventData.ghRepoFullName = event.GetRepo().GetFullName()
		eventData.ghBranch = event.GetPullRequest().GetBase().GetRef()
		eventData.imageTag = "latest"
	default:
		return nil, fmt.Errorf("unsupported event type: %v", reflect.TypeOf(event))
	}

	eventData.imageName = s.getImageName(eventData.ghRepoFullName)
	log.Debugf("Image name: %s, image tag: %s\n", eventData.imageName, eventData.imageTag)

	return eventData, nil
}

func (s *Server) getImageName(repoFullName string) string {
	if s.Options.ImageSuffix != "" {
		return fmt.Sprintf("%s-%s", repoFullName, s.Options.ImageSuffix)
	}
	return repoFullName
}

func (s *Server) getGithubRepo(ghRepoFullName, ghBranch string) error {
	return s.GithubClient.DownloadGithubRepository(s.Options.LocalRepoDir, ghRepoFullName, ghBranch)
}

func (s *Server) handleKustomization(ns string) ([]string, error) {
	deploykubeResPath := filepath.Join(s.Options.LocalRepoDir, s.Options.KubeResDir, ns)
	kustomizer := client.NewKustomizer(deploykubeResPath)
	return kustomizer.Build()
}

func (s *Server) issueCommentEventDeploy(eventData *eventData, kubeResources *[]string) error {
	// Build and push container image by handler
	log.Infof("Build and push the container image for %s enviroment...", eventData.namespace)
	if err := s.handleContainerization("deploy", eventData.ghLoginOwner, eventData.imageName, eventData.imageTag); err != nil {
		return err
	}
	log.Info("Build and push container image finished!")
	// Deploy to kubernetes
	log.Infof("Deploy the resources on Kubernetes for %s enviroment...", eventData.namespace)
	// Handle kubernetes deployment
	return s.deployKubeResources(eventData, kubeResources)
}

func (s *Server) issueCommentEventCleanup(eventData *eventData, kubeResources *[]string) error {
	// Clean up the deployment on kubernetes by handler
	log.Infof("Delete the deployment on Kubernetes for %s enviroment...", eventData.namespace)
	// Clean up kubernetes resources
	if err := s.cleanupKubeResoureces(eventData, kubeResources); err != nil {
		return err
	}
	log.Infof("Deleting the deployment on Kubernetes for %s enviroment is finished!", eventData.namespace)

	log.Infof("Delete the container image and repository for %s enviroment...", eventData.namespace)
	// Clean up local container image
	if err := s.handleContainerization("delete", eventData.ghLoginOwner, eventData.imageName, eventData.imageTag); err != nil {
		return err
	}
	// Clean up local source repository
	if err := s.cleanupLocalRepository(); err != nil {
		return err
	}
	// Clean up container image on Github packages
	return s.cleanupImageOnGithub(eventData.ctx, eventData.ghLoginOwner, eventData.imageName, eventData.imageTag)
}

func (s *Server) pullRequestEventDeploy(eventData *eventData, kubeResources *[]string) error {
	// Build and push container image by handler
	log.Infof("Build and push the container image for %s enviroment...", eventData.namespace)
	if err := s.handleContainerization("deploy", eventData.ghLoginOwner, eventData.imageName, eventData.imageTag); err != nil {
		return err
	}
	log.Info("Build and push container image finished!")
	// Deploy to kubernetes
	log.Infof("Deploy the resources on Kubernetes for %s enviroment...", eventData.namespace)
	// Handle kubernetes deployment
	return s.deployKubeResources(eventData, kubeResources)
}

func (s *Server) pullRequestEventCleanup(eventData *eventData) error {
	log.Infof("Delete container image for %s enviroment...", eventData.namespace)
	if err := s.handleContainerization("delete", eventData.ghLoginOwner, eventData.imageName, eventData.imageTag); err != nil {
		return err
	}
	// Clean up local source repository
	return s.cleanupLocalRepository()
}

func (s *Server) handleContainerization(action, ghLoginOwner, imageName, imageTag string) error {
	switch action {
	case "delete":
		// Cleanup local container image
		return s.DockerClient.ImageDelete(ghLoginOwner, imageName, imageTag)
	case "deploy":
		// Build the container image
		if err := s.DockerClient.ImageBuild(ghLoginOwner, imageName, imageTag, s.Options.LocalRepoDir); err != nil {
			return err
		}
		// Push the container image
		return s.DockerClient.ImagePush(ghLoginOwner, imageName, imageTag)
	}
	return nil
}

func (s *Server) cleanupLocalRepository() error {
	// Clean up local repository
	return s.GithubClient.DeleteLocalRepository(s.Options.LocalRepoDir)
}

func (s *Server) cleanupImageOnGithub(ctx context.Context, ghLoginOwner, imageName, imageTag string) error {
	packageType := "container"
	log.Infof("Deleting the package image %s:%s on Github...", imageName, imageTag)
	return s.GithubClient.DeletePackageImage(ctx, ghLoginOwner, packageType, imageName, imageTag)
}

func (s *Server) deployKubeResources(eventData *eventData, kubeResources *[]string) error {
	// Deploy namespace
	for _, res := range *kubeResources {
		if strings.Contains(res, "Namespace") {
			log.Debugf("found Namespace file:\n%s\n", res)
			if _, _, err := s.KubeClient.Deploy(eventData.ctx, []byte(res), eventData.namespace); err != nil {
				return fmt.Errorf("failed to deploy namespace: %v", err)
			}
			break
		}
	}
	// TODO: trigger github workflow to deploy kubernetes secrets

	// Deploy resources
	for _, res := range *kubeResources {
		if strings.Contains(res, "Namespace") {
			continue
		}
		if strings.Contains(res, "Kind: Deployment") && eventData.imageTag != "latest" {
			res = strings.Replace(res, "latest", eventData.imageTag, -1)
		}
		log.Debugf("Deploying resource:\n%s\n", res)
		if _, _, err := s.KubeClient.Deploy(eventData.ctx, []byte(res), eventData.namespace); err != nil {
			return err
		}
	}
	log.Info("Deployment completed!")
	return nil
}

func (s *Server) cleanupKubeResoureces(eventData *eventData, kubeResources *[]string) error {
	for _, res := range *kubeResources {
		if strings.Contains(res, "kind: Deployment") {
			res = strings.Replace(res, "latest", eventData.imageTag, -1)
		}
		log.Debugf("Delete resource:\n%s\n", res)
		if err := s.KubeClient.Delete(eventData.ctx, []byte(res), eventData.namespace); err != nil {
			return err
		}
	}
	log.Info("Cleanup completed!")
	return nil
}
