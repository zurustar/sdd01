#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") -d <database> -o <output-dir> [-k <retention>]

Options:
  -d, --database   Path to the SQLite database file to back up.
  -o, --output     Directory where backups are stored.
  -k, --keep       Number of most recent backups to keep (default: 7).
USAGE
}

DB_PATH=""
DEST_DIR=""
RETENTION=7

while [[ $# -gt 0 ]]; do
  case "$1" in
    -d|--database)
      DB_PATH="$2"
      shift 2
      ;;
    -o|--output)
      DEST_DIR="$2"
      shift 2
      ;;
    -k|--keep)
      RETENTION="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$DB_PATH" || -z "$DEST_DIR" ]]; then
  echo "Database path and output directory are required." >&2
  usage
  exit 1
fi

if [[ ! -f "$DB_PATH" ]]; then
  echo "Database file not found: $DB_PATH" >&2
  exit 1
fi

mkdir -p "$DEST_DIR"

TIMESTAMP=$(date -u +%Y%m%d%H%M%SZ)
BASENAME=$(basename "$DB_PATH")
TMP_FILE=$(mktemp "$DEST_DIR/${BASENAME}.${TIMESTAMP}.XXXXXX")
OUTPUT_BASE="$DEST_DIR/${BASENAME}.${TIMESTAMP}.sqlite3"

if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DB_PATH" ".backup '$TMP_FILE'"
else
  cp "$DB_PATH" "$TMP_FILE"
fi

gzip -f "$TMP_FILE"
mv "$TMP_FILE.gz" "${OUTPUT_BASE}.gz"

echo "Backup written to ${OUTPUT_BASE}.gz"

if [[ "$RETENTION" =~ ^[0-9]+$ && "$RETENTION" -gt 0 ]]; then
  mapfile -t BACKUPS < <(ls -1t "$DEST_DIR"/"$BASENAME".*.sqlite3.gz 2>/dev/null || true)
  if [[ ${#BACKUPS[@]} -gt "$RETENTION" ]]; then
    for OLD in "${BACKUPS[@]:$RETENTION}"; do
      rm -f "$OLD"
    done
  fi
fi
