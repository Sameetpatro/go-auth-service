#!/usr/bin/env bash
# Wipe backend data and seed 50 Indian guests + coordinators.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export CSV_FILE="${CSV_FILE:-${SCRIPT_DIR}/seed_guests.csv}"
export COORDINATORS="${COORDINATORS:-2}"

# Prefer direct DB reset when DATABASE_URL is set (full wipe including coordinators).
if [ -n "${DATABASE_URL:-}" ] || grep -q '^DATABASE_URL=.' "${SCRIPT_DIR}/../.env" 2>/dev/null; then
  echo "Resetting via database..."
  (cd "${SCRIPT_DIR}/.." && go run ./cmd/reset)
else
  "${SCRIPT_DIR}/reset_remote.sh"
fi

"${SCRIPT_DIR}/seed_remote.sh"

echo ""
echo "Done. QR images are served at:"
echo "  ${BASE_URL:-https://go-auth-service-brbh.onrender.com}/storage/qr/{guest-uuid}.png"
echo "List all QR URLs:"
echo "  make list-qrs"
