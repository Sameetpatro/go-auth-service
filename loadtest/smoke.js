// Smoke test: 1 VU sanity check of the core endpoints.
// Run: k6 run -e BASE_URL=http://localhost:8080 loadtest/smoke.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, MASTER_EMAIL, MASTER_PASSWORD, login, authHeaders } from './lib.js';

export const options = {
  vus: 1,
  iterations: 5,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<2000'],
  },
};

export function setup() {
  return { token: login(MASTER_EMAIL, MASTER_PASSWORD) };
}

export default function (data) {
  const headers = authHeaders(data.token);

  const health = http.get(`${BASE_URL}/health`, { tags: { endpoint: 'health' } });
  check(health, { 'health 200': (r) => r.status === 200 });

  const guests = http.get(`${BASE_URL}/api/v1/guests?page=1&per_page=20`, {
    headers,
    tags: { endpoint: 'list_guests' },
  });
  check(guests, { 'guest list 200': (r) => r.status === 200 });

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

  // If any guest exists, fetch its QR image (exercises card render/CDN path).
  const first = guests.json('data.data.0');
  if (first) {
    const qr = http.get(`${BASE_URL}/api/v1/guests/${first.id}/qr-image`, {
      headers,
      tags: { endpoint: 'qr_image' },
    });
    check(qr, {
      'qr image 200': (r) => r.status === 200,
      'qr image is png': (r) => String(r.headers['Content-Type']).includes('image/png'),
    });
  }

  sleep(1);
}
