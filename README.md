# Event Entry Management System - Backend

Production-grade REST API for QR-based event entry management, built with Go, Gin, PostgreSQL, and JWT authentication.

## Features

- **JWT Authentication** — Access tokens (24h) and refresh tokens (30 days)
- **Role-Based Access Control** — Master and Coordinator roles
- **Guest Management** — CRUD, CSV/XLSX import, dynamic metadata fields
- **QR Code Generation** — Signed tokens with PNG images
- **Atomic Scan Validation** — `SELECT FOR UPDATE` prevents duplicate entries
- **WhatsApp Invitations** — Pluggable notification service (WhatsApp, Email, SMS)
- **Analytics & Insights** — Dashboard metrics, hourly counts, coordinator stats
- **Real-Time Updates** — WebSocket events for live dashboard
- **Audit Logging** — All critical actions tracked
- **Report Export** — CSV and PDF exports
- **Swagger Documentation** — Interactive API docs

## Quick Start

### Prerequisites

- Go 1.23+
- PostgreSQL 14+
- Docker (optional)

### Local Development

1. **Clone and configure**

```bash
cp .env.example .env
```

2. **Start PostgreSQL**

```bash
docker compose up postgres -d
```

3. **Run migrations**

Apply `migrations/001_initial_schema.up.sql` to your PostgreSQL database.

4. **Run the server**

```bash
go run ./cmd/server
```

5. **Access**

- API: http://localhost:8080
- Swagger: http://localhost:8080/swagger/index.html
- WebSocket: ws://localhost:8080/ws
- Health: http://localhost:8080/health

### Default Master Account

| Field    | Value              |
|----------|--------------------|
| Email    | master@event.app   |
| Password | Master@123         |



This starts PostgreSQL and the API server with automatic schema initialization.

## API Overview

### Authentication

| Method | Endpoint              | Description        |
|--------|-----------------------|--------------------|
| POST   | /api/v1/auth/login    | Login              |
| POST   | /api/v1/auth/refresh  | Refresh token      |

### Guests

| Method | Endpoint                  | Role              | Description          |
|--------|---------------------------|-------------------|----------------------|
| GET    | /api/v1/guests            | Master, Coordinator | List guests        |
| POST   | /api/v1/guests            | Master, Coordinator | Create guest       |
| GET    | /api/v1/guests/:id        | Master, Coordinator | Get guest          |
| PUT    | /api/v1/guests/:id        | Master            | Update guest         |
| DELETE | /api/v1/guests/:id        | Master            | Delete guest         |
| GET    | /api/v1/guests/search?q=  | Master, Coordinator | Search guests      |
| GET    | /api/v1/guests/verify?q=  | Master, Coordinator | Manual verification|
| POST   | /api/v1/guests/import     | Master, Coordinator | CSV/XLSX import    |

### Scan

| Method | Endpoint       | Description                    |
|--------|----------------|--------------------------------|
| POST   | /api/v1/scan   | Validate QR and check in guest |

**Scan Response Values:** `ENTRY_ALLOWED`, `ALREADY_ENTERED`, `ENTRY_DENIED`

### Coordinators (Master Only)

| Method | Endpoint                                | Description           |
|--------|-----------------------------------------|-----------------------|
| POST   | /api/v1/coordinators                    | Create coordinator    |
| GET    | /api/v1/coordinators                    | List coordinators     |
| PATCH  | /api/v1/coordinators/:id/disable        | Disable coordinator   |
| POST   | /api/v1/coordinators/:id/reset-password | Reset password        |

### Analytics & Reports

| Method | Endpoint                        | Description          |
|--------|---------------------------------|----------------------|
| GET    | /api/v1/analytics/dashboard     | Dashboard metrics    |
| GET    | /api/v1/insights                | Insights data        |
| GET    | /api/v1/reports/export/csv      | CSV export           |
| GET    | /api/v1/reports/export/pdf      | PDF export           |

## Project Structure

```
cmd/server/          Application entry point
internal/
  auth/              JWT token service
  audit/             Audit logging
  config/            Environment configuration
  database/          PostgreSQL connection
  dto/               Request/response DTOs
  guests/            CSV/XLSX import parser
  handler/           HTTP handlers
  middleware/        Auth, CORS, rate limiting
  models/            Domain models
  notifications/     Pluggable notification providers
  qr/                QR token & image generation
  repository/        Database repositories
  router/            Route definitions
  service/           Business logic
  websocket/           Real-time event hub
migrations/          SQL migrations
pkg/                 Shared utilities
docs/                Swagger generated docs
```

## Guest Import Format

CSV/XLSX files must include a header row. Core fields:

| Column       | Required |
|--------------|----------|
| name         | Yes      |
| phone_number | No       |
| email        | No       |

Additional columns are stored in the `metadata` JSONB field for future extensibility (e.g., `vip_status`, `table_number`).

## WebSocket Events

Connect to `ws://localhost:8080/ws` to receive real-time events:

- `guest_checked_in` — Guest successfully checked in
- `new_guest_added` — New guest created
- `coordinator_created` — New coordinator account
- `dashboard_updated` — Analytics data changed
- `insights_updated` — Insights data changed

## Production Deployment

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for detailed production deployment instructions.

## Security

- Bcrypt password hashing
- JWT with separate access/refresh secrets
- Rate limiting (configurable RPM)
- Role-based authorization middleware
- SQL injection protection via parameterized queries
- Secure HTTP headers
- CORS configuration
- Input validation on all endpoints
