#!/usr/bin/env bash
# Seed guests and coordinators on a remote deployment via the REST API.
set -euo pipefail

BASE_URL="${BASE_URL:-https://go-auth-service-brbh.onrender.com}"
EMAIL="${SEED_EMAIL:-master@event.app}"
PASSWORD="${SEED_PASSWORD:-Master@123}"
CSV_FILE="${CSV_FILE:-scripts/seed_guests.csv}"
COORDINATORS="${COORDINATORS:-2}"

echo "Logging in to ${BASE_URL}..."
LOGIN_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")

TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])")

echo "Importing guests from ${CSV_FILE}..."
IMPORT_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/guests/import" \
  -H "Authorization: Bearer ${TOKEN}" \
  -F "file=@${CSV_FILE}")

echo "$IMPORT_RESP" | python3 -c "
import sys, json
d = json.load(sys.stdin)['data']
print(f\"Imported {d['imported']}/{d['total_rows']} guests\")
for e in d.get('errors') or []:
    print(f'  - {e}')
"

echo "Creating coordinators (skips if ${COORDINATORS} already exist)..."
EXISTING=$(curl -sf "${BASE_URL}/api/v1/coordinators" -H "Authorization: Bearer ${TOKEN}")
EXISTING_COUNT=$(echo "$EXISTING" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data') or []))")

if [ "$EXISTING_COUNT" -ge "$COORDINATORS" ]; then
  echo "Already have ${EXISTING_COUNT} coordinator(s) — skipping creation"
  echo "$EXISTING" | python3 -c "
import sys, json
for c in json.load(sys.stdin).get('data') or []:
    print(f\"  - {c['email']} ({'active' if c['is_active'] else 'disabled'})\")
"
else
  TO_CREATE=$((COORDINATORS - EXISTING_COUNT))
  echo "Creating ${TO_CREATE} coordinator(s)..."
  for i in $(seq 1 "$TO_CREATE"); do
    RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/coordinators" \
      -H "Authorization: Bearer ${TOKEN}")
    echo "$RESP" | python3 -c "
import sys, json
d = json.load(sys.stdin)['data']
print(f\"  Coordinator: email={d['email']} password={d['password']}\")
"
  done
  echo "Save coordinator passwords — they are shown only once."
fi
