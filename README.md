# 🎓 Motzklist Backend

The backend service for the **Motzklist** project — a school equipment shopping platform built with Go. This API gateway manages schools, grades, equipment catalogs, user authentication, shopping carts, and payment processing through Stripe integration.

**Stack:** Go 1.25.4 | PostgreSQL | Stripe API | Docker

---

## 📋 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Local Development](#local-development)
  - [Docker Setup](#docker-setup)
- [API Documentation](#api-documentation)
- [Database Schema](#database-schema)
- [Configuration](#configuration)
- [Testing](#testing)
- [Deployment](#deployment)
- [Contributing](#contributing)

---

## 🎯 Overview

Motzklist Backend is a RESTful API service that powers the Motzklist web platform. It provides endpoints for:

- **School & Grade Management**: Retrieve and manage educational institutions and grade levels
- **Equipment Catalogs**: Browse required equipment for different schools and grades
- **User Authentication**: Session-based authentication with secure login/logout
- **Shopping Cart**: Save and manage shopping lists
- **Payment Processing**: Stripe integration for secure checkout and order management
- **Admin Panel**: Manage schools, grades, equipment, and view payment history

All responses support **multi-language localization** (English & Hebrew) via the `lang` query parameter.

---

## ✨ Features

- ✅ **RESTful API** with JSON request/response format
- ✅ **Multi-language Support** — Localized content for English and Hebrew
- ✅ **User Authentication** — Session-based authentication with secure cookies
- ✅ **Shopping Cart Management** — Persistent cart storage per user
- ✅ **Stripe Integration** — Secure payment processing and refund management
- ✅ **Admin Endpoints** — Full CRUD operations for schools, grades, and equipment
- ✅ **CORS Support** — Configurable cross-origin requests
- ✅ **Docker Containerization** — Easy deployment with Docker & Docker Compose
- ✅ **PostgreSQL Database** — Relational database with multi-language support
- ✅ **Unit Tests** — Comprehensive test coverage for core functionality

---

## 📁 Project Structure

```
.
├── main.go                  # Application entry point, HTTP routing
├── main_test.go            # Unit tests for all handlers
├── db_api.go               # Database operations (schools, grades, equipment)
├── user_handlers.go        # Authentication endpoints (login, logout, auth status)
├── class_handlers.go       # School/Grade/Equipment endpoints
├── cart_handlers.go        # Shopping cart endpoints
├── payment_handlers.go     # Stripe checkout session creation
├── stripe_handlers.go      # Stripe webhook handling and admin payment endpoints
├── admin_handlers.go       # Admin CRUD operations for schools, grades, equipment
├── mock_db.go              # Mock data for development/testing
├── go.mod                  # Go module dependencies
├── go.sum                  # Go module checksums
├── Makefile                # Build and test commands
├── Dockerfile              # Docker image configuration
├── docker-compose.yml      # PostgreSQL + Backend orchestration
├── SCHEMA.md               # Complete API schema documentation
└── LICENSE                 # License file
```

---

## 🚀 Getting Started

### Prerequisites

- **Go 1.25.4+** — [Download](https://golang.org/dl/)
- **PostgreSQL 13+** — For database
- **Stripe Account** — For payment processing (optional for development)

### Local Development

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Motzklist/Back-End.git
   cd Back-End
   ```

2. **Download dependencies:**
   ```bash
   make deps
   ```

3. **Set up environment variables:**
   Create a `.env` file in the project root (or export these variables):
   ```bash
   # Database
   DATABASE_URL=postgres://user:password@localhost:5432/motzklist

   # Frontend
   CLIENT_ORIGIN=http://localhost:3000

   # Stripe (optional)
   STRIPE_SECRET_KEY=sk_test_...
   STRIPE_WEBHOOK_SECRET=whsec_...
   FRONTEND_URL=http://localhost:3000
   ```

4. **Set up the PostgreSQL database:**
   ```sql
   CREATE DATABASE motzklist;
   -- Run schema migrations (see SCHEMA.md for table definitions)
   ```

5. **Run the server:**
   ```bash
   make run
   ```
   The API will be available at `http://localhost:8080`

### Docker Setup

1. **Start services with Docker Compose:**
   ```bash
   docker-compose up --build
   ```
   This starts:
   - PostgreSQL database (port 5432)
   - Go backend service (port 8080)

2. **Verify the service:**
   ```bash
   curl http://localhost:8080/api/schools
   ```

---

## 📚 API Documentation

See [SCHEMA.md](SCHEMA.md) for the complete API specification.

### Example Endpoints

**Get Schools:**
```bash
GET /api/schools?lang=en
```

**Get Grades for a School:**
```bash
GET /api/grades?school_id=1&lang=en
```

**Get Equipment List:**
```bash
GET /api/equipment?school_id=1&grade_id=9&lang=en
```

**User Login:**
```bash
POST /api/login
Content-Type: application/json

{
  "username": "avner",
  "password": "2004"
}
```

**Get Shopping Cart:**
```bash
GET /api/cart?userid=1&lang=en
```

**Create Stripe Checkout Session:**
```bash
POST /api/checkout
Content-Type: application/json

{
  "userId": "1",
  "gradeId": "9",
  "items": [
    {
      "equipmentId": "101",
      "name": "Notebook",
      "quantity": 5,
      "amount": 250
    }
  ]
}
```

**Admin: List Schools (requires authentication):**
```bash
GET /api/admin/schools
Cookie: sessionid=...
```

---

## 🗄️ Database Schema

The database includes the following tables:

- **school** — Educational institutions with multi-language names
- **grade** — Grade levels (9-12) with multi-language names
- **equipment** — Product catalog with pricing and descriptions
- **requirement** — Links equipment to specific grade/school combinations
- **users** — User credentials (username, password, user ID)
- **cart** — Persistent shopping carts per user
- **orders** — Purchase history with order details
- **order_item** — Line items within orders (from Stripe webhooks)

See [SCHEMA.md](SCHEMA.md) for detailed table definitions and relationships.

---

## ⚙️ Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | ✅ | PostgreSQL connection string |
| `CLIENT_ORIGIN` | ✅ | Frontend origin for CORS (e.g., `http://localhost:3000`) |
| `STRIPE_SECRET_KEY` | ❌ | Stripe API secret key |
| `STRIPE_WEBHOOK_SECRET` | ❌ | Stripe webhook signing secret |
| `FRONTEND_URL` | ❌ | Frontend URL for Stripe success/cancel redirects |

### Database Connection

Update `DATABASE_URL` in your `.env`:
```
DATABASE_URL=postgres://username:password@localhost:5432/motzklist
```

### CORS Configuration

The `CLIENT_ORIGIN` environment variable accepts comma-separated origins:
```
CLIENT_ORIGIN=http://localhost:3000,https://motzklist.com
```

---

## 🧪 Testing

Run the test suite:

```bash
make test
```

To run tests with verbose output:
```bash
go test -v ./...
```

**Test Coverage:**
- Authentication (login, logout, session management)
- Shopping cart operations (GET, POST, validation)
- Equipment and grade retrieval
- Error handling and input validation

Some tests require a seeded test database and are marked with `t.Skip()`. To enable them, set up a test database and remove the skip statements.

---

## 🐳 Deployment

### Docker Build

Build the image:
```bash
docker build -t motzklist-backend:latest .
```

Run the container:
```bash
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://..." \
  -e CLIENT_ORIGIN="https://motzklist.com" \
  -e STRIPE_SECRET_KEY="sk_..." \
  motzklist-backend:latest
```

### Docker Compose

Deploy the full stack:
```bash
docker-compose up -d
```

View logs:
```bash
docker-compose logs -f backend
```

---

## 🛠️ Build Commands

| Command | Description |
|---------|-------------|
| `make run` | Start the development server |
| `make build` | Compile the binary to `./build/motzklist-backend` |
| `make clean` | Remove the build directory |
| `make deps` | Download Go dependencies |
| `make test` | Run the test suite |

---

## 📝 Contributing

1. **Fork** the repository
2. **Create a feature branch** (`git checkout -b feature/your-feature`)
3. **Make changes** and add tests
4. **Run tests** (`make test`) to ensure everything passes
5. **Commit** with clear messages (`git commit -am 'Add feature'`)
6. **Push** to your fork and **create a Pull Request**

### Code Standards

- Follow Go naming conventions and idioms
- Add tests for new functionality
- Update [SCHEMA.md](SCHEMA.md) if API changes are made
- Ensure CORS and multi-language support for new endpoints

---

## 📄 License

This project is licensed under the LICENSE file in this repository.

---

## 🙋 Support

For issues, questions, or suggestions:
- Open an [issue](https://github.com/Motzklist/Back-End/issues) on GitHub
- Check [SCHEMA.md](SCHEMA.md) for API documentation
- Review test files for usage examples

---

**Happy coding! 🚀**
