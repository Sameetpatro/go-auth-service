# Production Deployment Guide

## Overview

This guide covers deploying the Event Entry Management API to production environments.

## Prerequisites

- PostgreSQL 14+ (managed service recommended: AWS RDS, Google Cloud SQL, or Supabase)
- Docker or a Linux VM with Go 1.23+
- Reverse proxy (Nginx or Caddy) with TLS
- Domain name with SSL certificate

## Environment Variables

Copy `.env.example` and set production values:

```bash
# CRITICAL: Generate strong secrets
JWT_ACCESS_SECRET=$(openssl rand -base64 48)
JWT_REFRESH_SECRET=$(openssl rand -base64 48)

ENVIRONMENT=production
SERVER_BASE_URL=https://api.yourdomain.com

DB_HOST=your-rds-endpoint.region.rds.amazonaws.com
DB_PORT=5432
DB_USER=event_app
DB_PASSWORD=<strong-password>
DB_NAME=event_entry
DB_SSLMODE=require

EVENT_NAME=Your Event Name
EVENT_DATE=2026-06-15
EVENT_LOCATION=Your Venue

QR_IMAGE_PATH=/var/app/storage/qr
QR_IMAGE_URL=https://api.yourdomain.com/storage/qr

CORS_ALLOWED_ORIGINS=https://your-android-app-domain.com
RATE_LIMIT_RPM=200
```

## Database Setup

1. Create the database and user:

```sql
CREATE USER event_app WITH PASSWORD 'your-password';
CREATE DATABASE event_entry OWNER event_app;
GRANT ALL PRIVILEGES ON DATABASE event_entry TO event_app;
```

2. Run migrations:

```bash
psql -h $DB_HOST -U event_app -d event_entry -f migrations/001_initial_schema.up.sql
```

3. Change the default master password immediately after first login.

## Docker Deployment

```bash
# Build and deploy
docker compose -f docker-compose.yml up -d --build

# View logs
docker compose logs -f api
```

For production, use a managed PostgreSQL instance instead of the bundled container. Update `docker-compose.yml` to remove the postgres service and point `DB_HOST` to your managed instance.

## Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/api.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }

    location /storage/ {
        proxy_pass http://127.0.0.1:8080/storage/;
    }
}
```

## Scaling Considerations

### Horizontal Scaling

- Run multiple API instances behind a load balancer
- Use sticky sessions for WebSocket connections, or deploy a Redis pub/sub adapter for WebSocket broadcasting
- PostgreSQL connection pooling via PgBouncer for high concurrency

### Scan Concurrency

The `SELECT FOR UPDATE` pattern in the scan repository ensures atomic check-ins even when multiple gates scan the same QR simultaneously. No additional locking infrastructure is required at the application level.

### Storage

For production QR image storage, replace local filesystem storage with S3/GCS:

1. Implement a `StorageProvider` interface in `internal/qr/`
2. Upload generated PNG files to cloud storage
3. Set `QR_IMAGE_URL` to your CDN URL

### Notifications

Replace stub notification providers with real integrations:

- **WhatsApp**: Meta Cloud API or WhatsApp Business API
- **Email**: SendGrid, AWS SES, or Mailgun
- **SMS**: Twilio

Implement the `notifications.Provider` interface for each provider.

## Monitoring

Recommended observability stack:

- **Logs**: Structured JSON logging to CloudWatch, Datadog, or ELK
- **Metrics**: Prometheus + Grafana for request latency, scan rates, error rates
- **Alerts**: High duplicate scan rate, failed login attempts, database connection errors

## Backup Strategy

- Daily automated PostgreSQL backups (RDS automated backups or pg_dump cron)
- Retain QR images in durable cloud storage with versioning
- Test restore procedures monthly

## Health Checks

```bash
curl https://api.yourdomain.com/health
# {"status":"ok"}
```

Configure your load balancer to use `/health` as the health check endpoint.

## Security Checklist

- [ ] Change default master password
- [ ] Generate unique JWT secrets (min 32 characters)
- [ ] Enable PostgreSQL SSL (`DB_SSLMODE=require`)
- [ ] Restrict CORS to your Android app domain
- [ ] Enable rate limiting appropriate for your event size
- [ ] Use TLS everywhere (HTTPS + WSS)
- [ ] Restrict database access to API servers only
- [ ] Rotate JWT secrets periodically
- [ ] Review audit logs regularly

## Swagger Documentation

Generate updated Swagger docs:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs
```

Access at: `https://api.yourdomain.com/swagger/index.html`

Consider restricting Swagger access in production via Nginx IP allowlist or basic auth.
