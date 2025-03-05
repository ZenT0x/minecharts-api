# Minecharts
Minecharts is a Go API for managing Minecraft server pods on a Kubernetes cluster using Gin and client-go. It is containerized with Docker and includes Kubernetes manifests for easy deployment.

## Features
  - RESTful API with Gin: Manage Minecraft server pods via a clean REST interface.
  - Kubernetes Integration: Use client-go to interact with your Kubernetes cluster.
  - Containerized with Docker: Easily build and deploy your application.
  - Hot Reload with Air: Enjoy a fast development cycle with automatic reloads.
  - Structured Git Workflow: Includes branches for stable releases, development, and features.

## Getting Started
### Prerequisites
  - Go (v1.24+)
  - Docker
  - A Kubernetes cluster (Minikube, Kind, or cloud provider)
  - Air (for hot reload, optional)

### Installation
Clone the repository:
```bash
git clone https://github.com/ZenT0x/minecharts.git
```

Go into the repository:
```bash
cd minecharts
```

Install dependencies:
```bash
go mod tidy
```

Run the application:
- Without hot reload:
    ```bash
    go run main.go
    ```
- With hot reload:
    ```bash
    air
    ```
# Git Workflow
  - main: Contains stable, production-ready code.
  - dev: Active development branch.
  - feature/*: Branches for new features or fixes.
  - Version branches: For maintaining published versions (e.g., 1.4).

# Roadmap
  - [ ] ClusterIP + IngressRoute (Traefik specific Ingress controller support)
  - [ ] ClusterIP + Ingress (All Ingress controller support)
  - [ ] NodePort (Expose server port directly)
  - [ ] LoadBalancer (Get Public IP for server)
  - [ ] Extend API endpoints for full Minecraft server management.
  - [ ] Develop a Kubernetes operator for automated resource handling.
  - [ ] Implement CI/CD pipelines for testing and deployment.
  - [ ] Build a web interface for easier management.
