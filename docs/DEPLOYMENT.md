# Production Deployment Guide

## Overview

This guide covers deploying the Event Entry Management API to production. The
recommended stack is:

- **API**: Render web service (Docker)
- **Database**: Neon Postgres (free tier)
- **QR image storage**: Cloudinary (free tier)

## Environment Variables

Copy `.env.example` and set production values:

```bash
# CRITICAL: Generate strong secrets
JWT_ACCESS_SECRET=$(openssl rand -base64 48)
JWT_REFRESH_SECRET=$(openssl rand -base64 48)

ENVIRONMENT=production
SERVER_BASE_URL=https://api.yourdomain.com

# Neon pooled connection string (see "Database Setup (Neon)" below)
DATABASE_URL=postgresql://user:password@ep-xxxxx-pooler.region.aws.neon.tech/event_entry?sslmode=require

# Cloudinary credential (see "QR Image Storage (Cloudinary)" below)
CLOUDINARY_URL=cloudinary://<api_key>:<api_secret>@<cloud_name>

AUTO_MIGRATE=true

EVENT_NAME=Your Event Name
EVENT_DATE=2026-06-15
EVENT_LOCATION=Your Venue

CORS_ALLOWED_ORIGINS=https://your-android-app-domain.com
RATE_LIMIT_RPM=200
```

## Database Setup (Neon)

1. Create a free project at [console.neon.tech](https://console.neon.tech).
   Pick the region closest to your Render service region.
2. In the project, create a database named `event_entry` (or use the default).
3. Copy the **pooled** connection string: Dashboard -> Connect ->
   check **"Connection pooling"**. The host contains `-pooler`, e.g.
   `ep-xxxxx-pooler.ap-southeast-1.aws.neon.tech`. Use the pooled endpoint —
   Neon's free tier allows few direct connections, and the app opens up to
   `DB_MAX_OPEN_CONNS` (default 25).
4. Ensure the string ends with `?sslmode=require` (Neon requires TLS; the app
   also defaults to `require` when `sslmode` is omitted).
5. Set it as `DATABASE_URL` in Render -> your service -> Environment.
6. Keep `AUTO_MIGRATE=true` for the first deploy — the app creates the full
   schema and seeds the master user (`master@event.app` / `Master@123`) on boot.
7. Change the default master password immediately after first login.

Notes for Neon free tier:

- The compute autosuspends after inactivity and resumes in a few hundred ms on
  the next query — no code changes needed.
- `DB_CONN_MAX_LIFETIME` (default 5m) keeps stale connections from lingering
  across autosuspend cycles.

## QR Image Storage (Cloudinary)

QR invitation cards are uploaded to Cloudinary so they survive service
restarts (Render's disk is ephemeral) and are served from a global CDN.

1. Create a free account at [cloudinary.com](https://cloudinary.com).
2. Dashboard -> API Keys -> copy the **API environment variable**, formatted
   `cloudinary://<api_key>:<api_secret>@<cloud_name>`.
3. Set it as `CLOUDINARY_URL` in Render.

Images are stored under public IDs `qr/guest_{id}` and overwritten when a
card is regenerated. If `CLOUDINARY_URL` is unset, the app falls back to
local-disk storage (fine for local dev; not for Render).

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

QR image storage is handled by Cloudinary (`CLOUDINARY_URL`) — see
"QR Image Storage (Cloudinary)" above. The uploader lives in
`internal/storage/cloudinary.go` behind the `qr.Uploader` interface; a
different provider (S3/GCS) can be swapped in by implementing `UploadPNG`.

### Load Testing

k6 scripts covering the 20-user event profile (5 leaders + 10 scanning
coordinators + 5 viewers) live in `loadtest/` — see `loadtest/README.md`.

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
