# Minecharts
Minecharts is a Go API for managing Minecraft server pods on a Kubernetes cluster using Gin and client-go. It is containerized with Docker and includes Kubernetes manifests for easy deployment.

## Features
    - RESTful API with Gin: Manage Minecraft server pods via a clean REST interface.
    - Kubernetes Integration: Use client-go to interact with your Kubernetes cluster.
    - Structured Git Workflow: Includes branches for stable releases, development, and features.
    - Based on itzg/docker-minecraft-server: Leverages the well-maintained and feature-rich Minecraft server Docker image.

## Getting Started
### Prerequisites
    - Go (v1.24+)
    - Docker
    - A Kubernetes cluster (Minikube, Kind, or cloud provider)

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
```bash
        go run main.go
```

### Docker Image

Pull the Docker image:
```bash
docker pull ghcr.io/zent0x/minecharts:latest
```

## Minecraft Server Image
This project uses the [itzg/docker-minecraft-server Docker](https://github.com/itzg/docker-minecraft-server) image to deploy Minecraft servers in Kubernetes. This image offers extensive customization options through environment variables, allowing you to configure various server types, versions, and plugins.

# Git Workflow
    - main: Contains stable, production-ready code.
    - dev: Active development branch.
    - feature/*: Branches for new features or fixes.
    - Version branches: For maintaining published versions (e.g., 1.4).

# Roadmap
    - [ ] Basic server management
        - [x] Create/delete Minecraft server instances (pods)
        - [ ] Start/stop/restart Minecraft server instances
    - [ ] Server customization
        - [ ] Support majority of environment variables for Docker image customization
        - [ ] Execute commands in running Minecraft servers
    - [ ] Networking options
        - [ ] ClusterIP + IngressRoute (Traefik specific Ingress controller support)
        - [ ] ClusterIP + Ingress (All Ingress controller support)
        - [ ] NodePort (Expose server port directly)
        - [ ] LoadBalancer (Get Public IP for server)
    - [ ] Extend API endpoints for full Minecraft server management
    - [ ] Develop a Kubernetes operator for automated resource handling
    - [ ] Implement CI/CD pipelines for testing and deployment
    - [ ] Build a web interface for easier management
