#!/bin/bash
# PostgreSQL backup script for Kiwari POS
# Backs up the database daily and retains last 30 days
#
# Cron setup example (run daily at 2 AM):
# 0 2 * * * /home/iqbal/docker/pos/backup.sh >> /home/iqbal/backups/pos/backup.log 2>&1

set -euo pipefail  # Exit on error, undefined vars, pipe failures

BACKUP_DIR="/home/iqbal/backups/pos"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Backup database
echo "Starting backup at $(date)"
docker exec pos-db pg_dump -U pos pos_db | gzip > "$BACKUP_DIR/pos_$DATE.sql.gz"

# Verify backup was created and is non-empty
if [ ! -s "$BACKUP_DIR/pos_$DATE.sql.gz" ]; then
    echo "ERROR: Backup file is empty or was not created"
    exit 1
fi

# Cleanup old backups (retain last 30 days)
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +30 -delete

echo "Backup completed: pos_$DATE.sql.gz"
echo "Backup size: $(du -h "$BACKUP_DIR/pos_$DATE.sql.gz" | cut -f1)"
