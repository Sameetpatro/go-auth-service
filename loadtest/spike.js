// Spike test: ramp from 0 to 40 VUs (2x the expected 20 users) to find the
// breaking point. Read-heavy mix — dashboard, registry, guest list, QR image.
// Run: k6 run -e BASE_URL=http://localhost:8080 loadtest/spike.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, MASTER_EMAIL, MASTER_PASSWORD, login, authHeaders } from './lib.js';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '30s', target: 40 },
    { duration: '60s', target: 40 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    // Looser than the event simulation: we want to observe degradation,
    // not abort the run.
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<5000'],
  },
};

export function setup() {
  const token = login(MASTER_EMAIL, MASTER_PASSWORD);

  // Grab one guest id (if any) so the spike also exercises QR rendering.
  const res = http.get(`${BASE_URL}/api/v1/guests?page=1&per_page=1`, {
    headers: authHeaders(token),
  });
  const first = res.status === 200 ? res.json('data.data.0') : null;
  return { token, guestId: first ? first.id : null };
}

export default function (data) {
  const headers = authHeaders(data.token);

  const dashboard = http.get(`${BASE_URL}/api/v1/analytics/dashboard`, {
    headers,
    tags: { endpoint: 'dashboard' },
  });
  check(dashboard, { 'dashboard 200': (r) => r.status === 200 });

  const registry = http.get(`${BASE_URL}/api/v1/guests/registry`, {
    headers,
    tags: { endpoint: 'registry' },
  });
  check(registry, { 'registry 200': (r) => r.status === 200 });

  const list = http.get(`${BASE_URL}/api/v1/guests?page=1&per_page=20`, {
    headers,
    tags: { endpoint: 'list_guests' },
  });
  check(list, { 'guest list 200': (r) => r.status === 200 });

  if (data.guestId) {
    const qr = http.get(`${BASE_URL}/api/v1/guests/${data.guestId}/qr-image`, {
      headers,
      tags: { endpoint: 'qr_image' },
    });
    check(qr, { 'qr image 200': (r) => r.status === 200 });
  }

  sleep(0.5 + Math.random());
}
