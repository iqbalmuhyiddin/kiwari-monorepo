#!/bin/bash
# PostgreSQL backup script for Kiwari POS (Production VPS)
# Backs up both pos_staging and pos_production databases
# Retains last 30 days
#
# Cron setup example (run daily at 2 AM):
# 0 2 * * * /home/iqbal/docker/postgres/backup.sh >> /home/iqbal/backups/pos/backup.log 2>&1

set -euo pipefail  # Exit on error, undefined vars, pipe failures

BACKUP_DIR="/home/iqbal/backups/pos"
DATE=$(date +%Y%m%d_%H%M%S)
CONTAINER_NAME="postgres"
PG_USER="${POSTGRES_USER:-postgres}"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "Starting backup at $(date)"

# Backup pos_production database
echo "Backing up pos_production..."
docker exec "$CONTAINER_NAME" pg_dump -U "$PG_USER" pos_production | gzip > "$BACKUP_DIR/pos_production_$DATE.sql.gz"

# Verify production backup
if [ ! -s "$BACKUP_DIR/pos_production_$DATE.sql.gz" ]; then
    echo "ERROR: Production backup file is empty or was not created"
    exit 1
fi

echo "Production backup completed: pos_production_$DATE.sql.gz"
echo "Production backup size: $(du -h "$BACKUP_DIR/pos_production_$DATE.sql.gz" | cut -f1)"

# Backup pos_staging database
echo "Backing up pos_staging..."
docker exec "$CONTAINER_NAME" pg_dump -U "$PG_USER" pos_staging | gzip > "$BACKUP_DIR/pos_staging_$DATE.sql.gz"

# Verify staging backup
if [ ! -s "$BACKUP_DIR/pos_staging_$DATE.sql.gz" ]; then
    echo "ERROR: Staging backup file is empty or was not created"
    exit 1
fi

echo "Staging backup completed: pos_staging_$DATE.sql.gz"
echo "Staging backup size: $(du -h "$BACKUP_DIR/pos_staging_$DATE.sql.gz" | cut -f1)"

# Cleanup old backups (retain last 30 days)
echo "Cleaning up old backups (retaining last 30 days)..."
find "$BACKUP_DIR" -name "pos_production_*.sql.gz" -mtime +30 -delete
find "$BACKUP_DIR" -name "pos_staging_*.sql.gz" -mtime +30 -delete

echo "Backup process completed successfully at $(date)"
