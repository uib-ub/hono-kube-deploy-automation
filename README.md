# hono-kube-deploy-automation
[![Build Status](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/testing.yaml/badge.svg?branch=main)](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/testing.yaml)

[![codecov](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation/graph/badge.svg?token=BK0QPWIF3P)](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation)

The purpose of this repository is to provide an automated way to deploy hono-api to Kubernetes using Githhub webhook events.

## Workflow Diagram

```mermaid
graph TD
    A[HTTP Server to Listen to GitHub Webhook Events] --> B[GitHub Webhook Events]
    B --> C[Process events by GitHub Go Client]
    C --> D[Issue Comment Event]
    C --> E[Pull Request Event]

    D --> F[Check Action: Deleted or Created/Edited]

    F -- Action: created/edited 'deploy dev' comment --> I[Clone GitHub repo]
    I --> J[Kustomize Kubernetes resources using Kustomize API]
    J --> K[Build and push Docker image using Docker Go Client]
    K --> L[Deploy to Dev Environment using Kubernetes Go Client and GitHub Go Client]

    F -- Action: deleted 'deploy dev' comment --> M[Delete Kubernetes resources using Kubernetes Go Client]
    M --> N[Delete local Docker image using Docker Go Client]
    N --> O[Delete local Git repo]
    O --> P[Delete image on GitHub Container Registry using GitHub Go Client]

    E --> Q[Check if PR is merged to main/master and closed]
    Q -- Yes -->  T[Clone GitHub repo]
    T --> U[Kustomize Kubernetes resources using Kustomize API]
    U --> V[Build Docker image with 'latest' image tag and push to GitHub Container Registry using Docker Go Client]
    V --> W[Deploy to Test Environment on Microk8s using Kubernetes Go Client and GitHub Go Client]
    W --> X[Cleanup: Delete local Docker image by Docker Go Client and delete local Git repo]

    subgraph Process Issue Comment Event
        F
        I
        J
        K
        L
        M
        N
        O
        P
    end

    subgraph Process Pull Request Event
        Q
        T
        U
        V
        W
        X
    end
