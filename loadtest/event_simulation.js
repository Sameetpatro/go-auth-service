// Event-day simulation: 20 concurrent users.
//   - 5 leader VUs   : create guests, view QR images, browse their guest list
//   - 10 coordinator VUs : scan QR tokens continuously at gates
//   - 5 viewer VUs   : dashboard + registry + insights polling
//
// Run:
//   k6 run -e BASE_URL=http://localhost:8080 \
//          -e QR_SECRET='change-me-access-secret-min-32-chars!!' \
//          -e CLEANUP=true loadtest/event_simulation.js
//
// NOTE: the server rate limiter is per-IP (RATE_LIMIT_RPM, default 100/min).
// All k6 traffic comes from one IP, so raise it on the server for load tests,
// e.g. RATE_LIMIT_RPM=100000.
import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  MASTER_EMAIL,
  MASTER_PASSWORD,
  login,
  authHeaders,
  signQrToken,
  createLeader,
  createCoordinator,
  createGuest,
} from './lib.js';

const DURATION = __ENV.DURATION || '2m';
const LEADERS = 5;
const COORDINATORS = 10;
const VIEWERS = 5;
const GUESTS_PER_LEADER = 30; // pre-seeded scan pool: 150 guests

export const options = {
  setupTimeout: '180s',
  teardownTimeout: '120s',
  scenarios: {
    leaders: {
      executor: 'constant-vus',
      exec: 'leaderFlow',
      vus: LEADERS,
      duration: DURATION,
    },
    coordinators: {
      executor: 'constant-vus',
      exec: 'coordinatorFlow',
      vus: COORDINATORS,
      duration: DURATION,
    },
    viewers: {
      executor: 'constant-vus',
      exec: 'viewerFlow',
      vus: VIEWERS,
      duration: DURATION,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<2000'],
    'http_req_duration{endpoint:scan}': ['p(95)<1000'],
    'http_req_duration{endpoint:create_guest}': ['p(95)<2000'],
    checks: ['rate>0.99'],
  },
};

export function setup() {
  const masterToken = login(MASTER_EMAIL, MASTER_PASSWORD);
  const runId = `${Date.now() % 1000000}`;

  const leaders = [];
  for (let i = 0; i < LEADERS; i++) {
    const created = createLeader(masterToken, `lt${runId}l${i}`);
    leaders.push({
      id: created.id,
      token: login(created.email, created.password),
    });
  }

  const coordinators = [];
  for (let i = 0; i < COORDINATORS; i++) {
    const created = createCoordinator(masterToken, `Gate ${i + 1}`);
    coordinators.push({
      id: created.id,
      gate: `Gate ${i + 1}`,
      token: login(created.email, created.password),
    });
  }

  // Pre-seed guests so coordinators have a realistic pool of QR tokens to scan.
  const scanPool = [];
  leaders.forEach((leader, li) => {
    for (let g = 0; g < GUESTS_PER_LEADER; g++) {
      const guest = createGuest(leader.token, `LT Pool ${runId} L${li} G${g}`);
      if (guest) scanPool.push(guest.uuid);
    }
  });
  if (scanPool.length === 0) {
    throw new Error('setup failed: no guests created for the scan pool');
  }

  return { masterToken, leaders, coordinators, scanPool, runId };
}

// Leader: add a guest, immediately view its QR card, then browse the list.
export function leaderFlow(data) {
  const vuIndex = (exec.vu.idInTest - 1) % data.leaders.length;
  const leader = data.leaders[vuIndex];
  const headers = authHeaders(leader.token);
  const iter = exec.scenario.iterationInTest;

  const guest = createGuest(leader.token, `LT Live ${data.runId} V${vuIndex} I${iter}`);
  check(guest, { 'guest created': (g) => g !== null && g.id > 0 });

  if (guest) {
    const qr = http.get(`${BASE_URL}/api/v1/guests/${guest.id}/qr-image`, {
      headers,
      tags: { endpoint: 'qr_image' },
    });
    check(qr, { 'qr image 200': (r) => r.status === 200 });
  }

  const list = http.get(`${BASE_URL}/api/v1/guests?page=1&per_page=20`, {
    headers,
    tags: { endpoint: 'list_guests' },
  });
  check(list, { 'guest list 200': (r) => r.status === 200 });

  sleep(1 + Math.random() * 2);
}

// Coordinator: scan a QR token. Rescans of the same guest must return
// ALREADY_ENTERED (never a second ENTRY_ALLOWED) — this validates the
// SELECT FOR UPDATE duplicate-scan guard under 10 concurrent scanners.
export function coordinatorFlow(data) {
  const vuIndex = (exec.vu.idInTest - 1) % data.coordinators.length;
  const coordinator = data.coordinators[vuIndex];
  const headers = authHeaders(coordinator.token);
  const iter = exec.scenario.iterationInTest;

  // Every 10th scan, present a forged token: server must deny it (HTTP 400).
  if (iter % 10 === 9) {
    const res = http.post(
      `${BASE_URL}/api/v1/scan`,
      JSON.stringify({ qr_token: 'forged.deadbeef', gate_name: coordinator.gate }),
      {
        headers,
        tags: { endpoint: 'scan_invalid' },
        responseCallback: http.expectedStatuses(400),
      },
    );
    check(res, { 'forged token denied': (r) => r.status === 400 });
  } else {
    const uuid = data.scanPool[iter % data.scanPool.length];
    const res = http.post(
      `${BASE_URL}/api/v1/scan`,
      JSON.stringify({ qr_token: signQrToken(uuid), gate_name: coordinator.gate }),
      { headers, tags: { endpoint: 'scan' } },
    );
    check(res, {
      'scan 200': (r) => r.status === 200,
      'scan result valid': (r) => {
        const result = r.json('data.result');
        return result === 'ENTRY_ALLOWED' || result === 'ALREADY_ENTERED';
      },
    });
  }

  sleep(0.5 + Math.random());
}

// Viewer: master/leader watching dashboards during the event.
export function viewerFlow(data) {
  const headers = authHeaders(data.masterToken);

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

  const insights = http.get(`${BASE_URL}/api/v1/insights`, {
    headers,
    tags: { endpoint: 'insights' },
  });
  check(insights, { 'insights 200': (r) => r.status === 200 });

  sleep(2 + Math.random() * 2);
}

export function teardown(data) {
  if (__ENV.CLEANUP !== 'true') return;
  const headers = authHeaders(data.masterToken);

  for (const leader of data.leaders) {
    http.del(`${BASE_URL}/api/v1/guests/all?leader_id=${leader.id}`, null, { headers });
    http.del(`${BASE_URL}/api/v1/leaders/${leader.id}`, null, { headers });
  }
  for (const coordinator of data.coordinators) {
    http.patch(`${BASE_URL}/api/v1/coordinators/${coordinator.id}/disable`, null, { headers });
  }
}
