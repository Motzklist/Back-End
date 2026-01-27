# Backend

This is the backend service for the project. It provides API endpoints and uses a mock database for testing purposes.

## Table of Contents
- [Project Structure](#project-structure)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Environment Variables](#environment-variables)

## Project Structure
- Dockerfile
- LICENSE
- Makefile
- README.md
- SCHEMA.md
- go.mod
- main.go
- mock_db.go

## Requirements
- Go 1.20+
- Docker (optional)

## Installation
Clone the repository:
```bash
git clone https://github.com/Motzklist/Back-End.git
cd Back-End
```

Install dependencies: 
```bash
go mod tidy
```

## Usage
### Running locally
```bash
go run main.go
```
The server will start on port 8080. You can configure the port using PORT environment variable.

### Using Docker
Build the Docker image:
```bash
docker build -t backend-service .
```
Run the container:
```bash
docker run -p 8080:8080 backend-service
```

## Environment Variables
- ```PORT``` – the port the server will run on (default: 8080)
- ```FRONTEND_URL``` – the frontend URL for CORS configuration
