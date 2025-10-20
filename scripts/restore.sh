#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") -d <database> -i <backup-file>

Options:
  -d, --database   Path to the SQLite database file to restore.
  -i, --input      Path to the backup file (.sqlite3 or .sqlite3.gz).
USAGE
}

DB_PATH=""
BACKUP_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -d|--database)
      DB_PATH="$2"
      shift 2
      ;;
    -i|--input)
      BACKUP_FILE="$2"
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

if [[ -z "$DB_PATH" || -z "$BACKUP_FILE" ]]; then
  echo "Database path and backup file are required." >&2
  usage
  exit 1
fi

if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

mkdir -p "$(dirname "$DB_PATH")"
TMP_FILE=$(mktemp)

if [[ "$BACKUP_FILE" == *.gz ]]; then
  gunzip -c "$BACKUP_FILE" > "$TMP_FILE"
else
  cp "$BACKUP_FILE" "$TMP_FILE"
fi

if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DB_PATH" ".restore '$TMP_FILE'"
else
  cp "$TMP_FILE" "$DB_PATH"
fi

echo "Database restored to $DB_PATH"
