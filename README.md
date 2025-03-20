# Minecharts
Minecharts is a Go API for managing Minecraft server pods on a Kubernetes cluster using Gin and client-go. It is containerized with Docker and includes Kubernetes manifests for easy deployment.

## Features
    - Create and delete Minecraft server instances (pods)
    - Start, stop, and restart Minecraft server instances
    - Customize server instances with environment variables
    - Execute commands in running Minecraft servers
    - And more to come!

## Getting Started
### Prerequisites
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

Apply the Kubernetes manifests:
```bash
kubectl apply -f kubernetes/
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
    - [x] Basic server management
        - [x] Create/delete Minecraft server instances (pods)
        - [x] Start/stop/restart Minecraft server instances
    - [x] Server customization
        - [x] Support majority of environment variables for Docker image customization
        - [x] Execute commands in running Minecraft servers
    - [x] Networking options
        - [x] ClusterIP (Internal server)
        - [x] NodePort (Expose server port directly)
        - [x] LoadBalancer (Get Public IP for server)
        - [x] MC Router
    - [ ] Extend API endpoints for full Minecraft server management
    - [ ] Develop a Kubernetes operator for automated resource handling
    - [ ] Implement CI/CD pipelines for testing and deployment
    - [ ] Build a web interface for easier management
