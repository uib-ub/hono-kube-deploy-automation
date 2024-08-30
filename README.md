# hono-kube-deploy-automation
[![Build Status](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/cicd.yaml/badge.svg?branch=main)](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/cicd.yaml) [![codecov](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation/graph/badge.svg?token=BK0QPWIF3P)](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation)


## Table of Contents

- [Overview](#Overview)
- [Features](#Features)
- [Workflow Diagram](#Workflow-diagram)
- [Configuration and Secrets](#Configuration-and-secrets)
- [Local Development with Docker Compose](#local-development-with-docker-compose)
- [Webhook Events](#webhook-events)
- [Testing and Code Coverage](#testing-and-code-coverage)
- [Deployment](#deployment)
- [Health Checks](#health-checks)

## Overview

This Go application is designed to automate the deployment of the `hono-api` to a Kubernetes cluster. It integrates with Go clients including GitHub, Docker, Kustomize, and Kubernetes to handle the deployment processes triggered by specific GitHub webhook events. The application listens for specific GitHub webhook events and triggers actions such as building Docker images, deploying Kubernetes resources, and managing workflows and package images.

## Features

- Automates deployment of the `hono-api` to Kubernetes clusters.
- Listens to GitHub webhook events for pull requests and issue comments.
- Supports building, pushing, and deleting Docker images locally and container registry.
- Utilizes Kustomize to build Kubernetes configuration resources.
- Deploys Kubernetes resources using the Kubernetes Go client.
- Integrates with Rollbar for error monitoring and logging.

## Workflow Diagram

```mermaid
flowchart TB
    A(HTTP Server to Listen to Webhook Events) --> B(GitHub Webhook Events)
    B --> C(Process events by GitHub Go Client)
    C --> D(Issue Comment Event)
    C --> E(Pull Request Event)

    D --> F(Check Action: Deleted or Created/Edited)

    F -- Action: created/edited 'deploy dev' comment --> G(Clone GitHub Repo)
    G --> H(Kustomize Kubernetes Resource using Kustomize API)
    H --> I(Build and Push Docker Image using Docker Go Client)
    I --> J(Deploy to Dev Environment using Kubernetes Go Client and GitHub Go Client)
    J -- Retry Mechanism --> J
    J --> K(Wait for all replicated pods running using Kubernetes Go Client)

    F -- Action: deleted 'deploy dev' comment --> L(Concurret Cleanup)
    L --> M(Delete Kubernetes Resource using Kubernetes Go Client)
    L --> N(Delete Local Docker Image using Docker Go Client)
    L --> O(Delete Local Git Repo)
    L --> P(Delete Image on GitHub Container Registry using GitHub Go Client)

    E --> Q(Check if PR is Merged to Main/Master and Closed)
    Q -- Yes -->  R(Clone GitHub Repo)
    R --> S(Kustomize Kubernetes Resource using Kustomize API)
    S --> T(Build Docker Image with 'latest' Tag and Push to GitHub Container Registry using Docker Go Client)
    T --> U(Deploy to Test Environment on Microk8s using Kubernetes Go Client and GitHub Go Client)
    U -- Retry Mechanism --> U
    U --> V(Wait for all replicated pods running using Kubernetes Go Client)
    V --> W(Concurret Cleanup)
    W --> X(Delete Local Docker Image using Docker Go Client)
    W --> Y(Delete Git Repo using Github Gl Client)

    subgraph Process Issue Comment Event
        F
        G
        H
        I
        subgraph Deploy to dev env
          J
        end
        K
        L
        subgraph Cleanup dev env
          M
          N
          O
          P
        end
    end

    subgraph Process Pull Request Event
        Q
        R
        S
        T
        subgraph Deploy to test env
          U
        end
        V
        W
        subgraph Cleanup test env
          X
          Y
        end
    end
```

## Configuration and Secrets

The application requires configuration and secret settings.
Configuration can be loaded from a file (config.yaml) 
The secrets and environment variables are handled by Github repository secrets and used in the Github workflow. 

Key secret:

- GitHub:
  - `GitHubToken`: GitHub personal access token for authentication.
  - `WebhookSecret`: Secret for verifying GitHub webhook payloads.

- Rollbar:
  - `RollbarToken`: Token for Rollbar error logging.

Key configuration:

- Github:
  - `workflowPrefix`: the prefix of the GitHub workflow name to deploy secrets to Kubernetes, such as "deploy-kube-secrets"
  - `localRep`: The local repository location, such as "app"
  - `packageType`: the GitHub package type, which is "container"

- Kubernetes:
  - `KubeConfig`: Path to the local kubeconfig file, if we run this Go application outside of the Kubernetes cluster.
  - `DevNamespace`: Kubernetes namespace for the development environment.
  - `TestNamespace`: Kubernetes namespace for the test environment.
  - `Resource`: Path to Kubernetes resource configuration directory ("microk8s-hono-api" for hono api).

- Container:
  - `Dockerfile`: Dockerfile file name.
  - `Registry`: container registry to push Docker images.
  - `ImageSuffix`: Suffix to append to Docker images.

## Local Development with Docker Compose

For local development, you can use the docker-compose.yaml file to build and run the application with ease. The docker-compose setup uses environment variables defined in the .env-template file. To get started:

1. Copy the .env-template to .env and configure the required environment variables

```
WEBHOOK_SECRET=test-secret
GITHUB_TOKEN=test-github-access-token
ROLLBAR_TOKEN=test-rollbar-token
MY_KUBECONFIG=/local-path-to/.kube/config
KUBECONFIG=/root/.kube/config
DOCKER_HOST=unix:///var/run/docker.sock
```

2. Run the application using Docker Compose:

```
docker-compose -f docker-compose.yaml up --build -d
```

for stop the application:

```
docker-compose -f docker-compose.yaml up down
```

3. use a tool such as [ngrok](https://ngrok.com/)

For example, run the following command:
```
ngrok http 8080
```

We get HTTP Endpoint:
```
Forwarding https://xxx.ngrok-free.app -> http://localhost:8080 
```

4. Copy `https://xxx.ngrok-free.app` to the GitHub webhook settings:
 - for the Payload URL, we give: "https://xxx.ngrok-free.app/webhook"
 - Content type: "application/json"
 - create a webhook secret and use it in the CICD workflow
 - Choose "Let me select individual events" and select "Issue comments" and "Pull requests"


## Webhook Events
The application handles the following GitHub webhook events:

a. Issue Comment Event: 

Deploy to the development environment when a comment "deploy dev" is created or deleted in a pull request.

b. Pull Request Event: 

Deploy to the test environment when a pull request labeled "type: deploy-hono-test" is merged into the main branch.

## Testing and Code Coverage

All Go client code (github.go, docker.go, kubernetes.go, and kustomize.go) is thoroughly tested with unit tests to ensure reliability. Code coverage is maintained using [Codecov](https://app.codecov.io/gh/uib-ub/hono-kube-deploy-automation), integrated via GitHub Actions to provide insights on test coverage.

## Deployment

Deployment is managed via a GitHub Actions CI/CD pipeline defined in CICD.yaml. This workflow automates testing, building, pushing Docker images, and deploying the application to a Kubernetes cluster.

## Health Checks

The application provides two health check endpoints:

* Liveness Probe: GET /health - Always returns 200 OK to indicate the application is alive.

* Readiness Probe: GET /ready - Returns 200 OK if the application is ready to handle requests, otherwise returns 503 Service Unavailable.
