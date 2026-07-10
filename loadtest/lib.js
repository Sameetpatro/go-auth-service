import http from 'k6/http';
import crypto from 'k6/crypto';
import { check } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const MASTER_EMAIL = __ENV.MASTER_EMAIL || 'master@event.app';
export const MASTER_PASSWORD = __ENV.MASTER_PASSWORD || 'Master@123';
// Must match the server's JWT_ACCESS_SECRET — QR tokens are HMAC-signed with it.
export const QR_SECRET = __ENV.QR_SECRET || 'change-me-access-secret-min-32-chars!!';

const JSON_HEADERS = { 'Content-Type': 'application/json' };

export function login(email, password) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    { headers: JSON_HEADERS, tags: { endpoint: 'login' } },
  );
  check(res, { 'login succeeded': (r) => r.status === 200 });
  if (res.status !== 200) {
    throw new Error(`login failed for ${email}: ${res.status} ${res.body}`);
  }
  return res.json('data.access_token');
}

export function authHeaders(token) {
  return { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` };
}

// Recreates the server-side QR token format: "{uuid}.{hmac_sha256_hex(uuid)}".
// This lets coordinator VUs "scan" guests without decoding QR PNGs.
export function signQrToken(guestUuid) {
  const sig = crypto.hmac('sha256', QR_SECRET, guestUuid, 'hex');
  return `${guestUuid}.${sig}`;
}

export function createLeader(masterToken, username) {
  const res = http.post(
    `${BASE_URL}/api/v1/leaders`,
    JSON.stringify({ username, generate_password: true }),
    { headers: authHeaders(masterToken), tags: { endpoint: 'create_leader' } },
  );
  if (res.status !== 200 && res.status !== 201) {
    throw new Error(`create leader failed: ${res.status} ${res.body}`);
  }
  return res.json('data'); // { id, email, password, ... }
}

export function createCoordinator(masterToken, gateName) {
  const res = http.post(
    `${BASE_URL}/api/v1/coordinators`,
    JSON.stringify({ gate_name: gateName }),
    { headers: authHeaders(masterToken), tags: { endpoint: 'create_coordinator' } },
  );
  if (res.status !== 200 && res.status !== 201) {
    throw new Error(`create coordinator failed: ${res.status} ${res.body}`);
  }
  return res.json('data'); // { id, email, password, assigned_gate, ... }
}

export function createGuest(leaderToken, name) {
  const res = http.post(
    `${BASE_URL}/api/v1/guests`,
    JSON.stringify({ name }),
    { headers: authHeaders(leaderToken), tags: { endpoint: 'create_guest' } },
  );
  if (res.status !== 200 && res.status !== 201) {
    return null;
  }
  return res.json('data'); // { id, uuid, ... }
}
