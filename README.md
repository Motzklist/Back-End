# Motzklist — Back-End

The API Gateway for the Motzklist project. This repository provides the gateway layer that routes requests to backend services, handles authentication, logging, and request validation.

### Tech stack
- Language: Go (Gin-Gonic)
- Container: Docker
- Build helper: Makefile

### Status
- Repository purpose: API Gateway for Motzklist
- TODO: fill in service-specific details (endpoints, env vars, diagrams)

## Table of contents
- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Getting started (local)](#getting-started-local)
- [Configuration](#configuration--environment-variables)
- [Build & run](#build--run-go)
- [Docker](#docker)
- [Tests & linting](#tests--linting)
- [Contributing](#contributing)
- [License](#license)


### Overview
This repository implements the HTTP API gateway for Motzklist. The gateway is responsible for:
- Routing HTTP requests to internal services
- Authentication and authorization checks
- Centralized logging and request tracing
- Rate-limiting and input validation (if configured)

### Features
- Gin-Gonic based HTTP server
- Configuration via environment variables and/or config file
- Dockerfile for container builds
- Makefile for common tasks (build, run, test)

### Architecture
<img src="https://github.com/Motzklist/Motzklist/blob/main/docs/assets/image7.png?raw=true" width="700" alt="Deployment Diagram">

### Requirements
- Go 1.20+ (adjust to your project's Go version)
- Docker (for container builds)
- GNU Make (for make targets)
- Optional: golangci-lint, delve (for debugging)

### Getting started (local)
1. Clone the repo:
   git clone https://github.com/Motzklist/Back-End.git
   cd Back-End

3. Build and run:
   make build
   ./bin/back-end

#### Common Make targets
- make build — compile the gateway binary
- make run — build then run locally (if present)
- make test — run unit tests
- make lint — run linter (golangci-lint)

#### Configuration / Environment variables
- PORT — HTTP port (default: 8080)
- GIN_MODE — gin run mode (debug|release)
- AUTH_SERVICE_URL — URL of authentication service
- LOG_LEVEL — debug|info|warn|error
- DB_URL / REDIS_URL — backend stores (if the gateway needs them)

#### Build & run (Go)
- Build:
  go build -o bin/back-end ./cmd
- Run:
  PORT=8080 ./bin/back-end

#### Docker
- Build image:
  docker build -t motzklist/back-end:latest .
- Run container:
  docker run --env-file .env -p 8080:8080 motzklist/back-end:latest

#### Tests & linting
- Run unit tests:
  go test ./...
- Lint:
  golangci-lint run

### Contributing
- Fork the repository, create a feature branch, and open a pull request.
- Follow the project's commit signoff and signature policies (see repository TODOs).
- Add tests for new functionality.

### License
- Apache License 2.0

