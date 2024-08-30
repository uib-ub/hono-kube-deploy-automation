# hono-kube-deploy-automation
[![Build Status](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/cicd.yaml/badge.svg?branch=main)](https://github.com/uib-ub/hono-kube-deploy-automation/actions/workflows/cicd.yaml) [![codecov](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation/graph/badge.svg?token=BK0QPWIF3P)](https://codecov.io/gh/uib-ub/hono-kube-deploy-automation)

The purpose of this repository is to provide an automated way to deploy hono-api to Kubernetes using Githhub webhook events.

## Workflow Diagram

```mermaid
graph TD
    A[HTTP Server to Listen to Webhook Events] --> B[GitHub Webhook Events]
    B --> C[Process events by GitHub Go Client]
    C --> D[Issue Comment Event]
    C --> E[Pull Request Event]

    D --> F[Check Action: Deleted or Created/Edited]

    F -- Action: created/edited 'deploy dev' comment --> G[Clone GitHub Repo]
    G --> H[Kustomize Kubernetes Resource using Kustomize API]
    H --> I[Build and Push Docker Image using Docker Go Client]
    I --> J[Deploy to Dev Environment using Kubernetes Go Client and GitHub Go Client]
    J -- Retry Mechanism --> J
    J --> K[Wait for all replicated pods running using Kubernetes Go Client]

    F -- Action: deleted 'deploy dev' comment --> L[Concurret Cleanup]
    L --> M[Delete Kubernetes Resource using Kubernetes Go Client]
    L --> N[Delete Local Docker Image using Docker Go Client]
    L --> O[Delete Local Git Repo]
    L --> P[Delete Image on GitHub Container Registry using GitHub Go Client]

    E --> Q[Check if PR is Merged to Main/Master and Closed]
    Q -- Yes -->  R[Clone GitHub Repo]
    R --> S[Kustomize Kubernetes Resource using Kustomize API]
    S --> T[Build Docker Image with 'latest' Tag and Push to GitHub Container Registry using Docker Go Client]
    T --> U[Deploy to Test Environment on Microk8s using Kubernetes Go Client and GitHub Go Client]
    U -- Retry Mechanism --> U
    U --> V[Wait for all replicated pods running using Kubernetes Go Client] 
    V --> W[Concurret Cleanup]
    W --> X[Delete Local Docker Image using Docker Go Client]
    W --> Y[Delete Git Repo using Github Gl Client]

    subgraph Process Issue Comment Event
        F
        G
        H
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
        R
        S
        T
        U
        V
        W
        X
        Y
    end
