# Load Tests (k6)

Validates the backend for the real event profile: **5 leaders** creating and
sharing guest QR codes plus **10 coordinators** scanning simultaneously
(20 concurrent users total).

## Prerequisites

- [k6](https://k6.io/docs/get-started/installation/) (`brew install k6`)
- A running server. Local stack:

```bash
docker compose up -d --build
```

- **Raise the rate limit for load tests.** The limiter is per-IP and all k6
  traffic comes from one IP, so the default `RATE_LIMIT_RPM=100` would throttle
  the test long before real load appears. For local docker compose:

```bash
RATE_LIMIT_RPM=100000 docker compose up -d --build
```

  (or add `RATE_LIMIT_RPM: "100000"` to the `api` service environment).

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `BASE_URL` | `http://localhost:8080` | Server under test |
| `MASTER_EMAIL` | `master@event.app` | Master login |
| `MASTER_PASSWORD` | `Master@123` | Master password |
| `QR_SECRET` | dev default | Server's `JWT_ACCESS_SECRET`; used to sign scan tokens |
| `CLEANUP` | unset | `true` deletes test leaders/guests in teardown |
| `DURATION` | `2m` | Steady-state duration of the event simulation |

`QR_SECRET` is needed because QR tokens are `{uuid}.{hmac_sha256(uuid, secret)}`
and the API (correctly) never returns raw tokens — the test signs them itself.

## Scripts

### 1. Smoke — sanity check (1 VU)

```bash
k6 run -e BASE_URL=http://localhost:8080 loadtest/smoke.js
```

### 2. Event simulation — the 20-user event-day profile

5 leaders (create guest, view QR, list), 10 coordinators (scan continuously,
including duplicate and forged-token scans), 5 dashboard viewers.

```bash
k6 run \
  -e BASE_URL=http://localhost:8080 \
  -e QR_SECRET='change-me-access-secret-min-32-chars!!' \
  -e CLEANUP=true \
  loadtest/event_simulation.js
```

Pass thresholds:

- error rate < 1%
- p95 latency < 2s overall, < 1s for scans
- every rescan returns `ALREADY_ENTERED`, never a duplicate `ENTRY_ALLOWED`

### 3. Spike — find the breaking point (0 → 40 VUs)

```bash
k6 run -e BASE_URL=http://localhost:8080 loadtest/spike.js
```

## Testing the deployed instance

Point `BASE_URL` at Render and use the production secrets:

```bash
k6 run \
  -e BASE_URL=https://go-auth-service-brbh.onrender.com \
  -e MASTER_PASSWORD='<real master password>' \
  -e QR_SECRET='<real JWT_ACCESS_SECRET>' \
  -e CLEANUP=true \
  loadtest/event_simulation.js
```

Note: the free Render instance cold-starts (~50s) after idle; hit `/health`
once and wait for a 200 before starting a run. Remember the production rate
limit also applies.
