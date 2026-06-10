#!/usr/bin/env bash
# Print guest names and QR image URLs from the API.
set -euo pipefail

BASE_URL="${BASE_URL:-https://go-auth-service-brbh.onrender.com}"
EMAIL="${SEED_EMAIL:-master@event.app}"
PASSWORD="${SEED_PASSWORD:-Master@123}"

LOGIN_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")

TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])")

GUESTS=$(curl -sf "${BASE_URL}/api/v1/guests?per_page=100" -H "Authorization: Bearer ${TOKEN}")

echo "$GUESTS" | python3 -c "
import sys, json
guests = json.load(sys.stdin).get('data', {}).get('data', [])
print(f'Total guests: {len(guests)}')
print()
for g in guests:
    qr = g.get('qr_image_url') or '(no image)'
    print(f\"{g['name']}: {qr}\")
"
