#!/usr/bin/env bash
# Reset all guest/coordinator data on a remote deployment (keeps master account).
set -euo pipefail

BASE_URL="${BASE_URL:-https://go-auth-service-brbh.onrender.com}"
EMAIL="${SEED_EMAIL:-master@event.app}"
PASSWORD="${SEED_PASSWORD:-Master@123}"

echo "Logging in to ${BASE_URL}..."
LOGIN_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")

TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])")

echo "Resetting all guest and coordinator data..."
HTTP_CODE=$(curl -s -o /tmp/reset_resp.json -w "%{http_code}" -X POST "${BASE_URL}/api/v1/admin/reset" \
  -H "Authorization: Bearer ${TOKEN}")

if [ "$HTTP_CODE" = "200" ]; then
  echo "Reset complete."
  cat /tmp/reset_resp.json | python3 -c "import sys,json; print(json.load(sys.stdin).get('message','OK'))"
  exit 0
fi

if [ "$HTTP_CODE" = "404" ]; then
  echo "Reset endpoint not deployed yet — falling back to deleting guests via API..."
  DELETED=0
  python3 <<PY
import json, urllib.request, sys

base = "${BASE_URL}"
token = "${TOKEN}"

def delete_guest(gid):
    req = urllib.request.Request(
        f"{base}/api/v1/guests/{gid}",
        headers={"Authorization": f"Bearer {token}"},
        method="DELETE",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status == 200
    except urllib.error.HTTPError:
        return False

def list_guests():
    req = urllib.request.Request(
        f"{base}/api/v1/guests?per_page=100",
        headers={"Authorization": f"Bearer {token}"},
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp)["data"]["data"]

deleted = 0
for _ in range(50):
    guests = list_guests()
    if not guests:
        break
    progress = False
    for g in guests:
        if delete_guest(g["id"]):
            deleted += 1
            progress = True
    if not progress:
        remaining = [g["name"] for g in list_guests()]
        print(f"Deleted {deleted} guest(s).")
        if remaining:
            print(f"Could not delete {len(remaining)} guest(s) via API: {', '.join(remaining)}")
            print("Use: DATABASE_URL=<external-url> go run ./cmd/reset")
            sys.exit(1)
        break
else:
    print(f"Deleted {deleted} guest(s).")
PY
  exit $?
fi

echo "Reset failed (HTTP ${HTTP_CODE})"
cat /tmp/reset_resp.json
exit 1
