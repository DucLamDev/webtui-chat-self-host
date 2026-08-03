FROM postgres:16-alpine

RUN apk add --no-cache \
      ca-certificates \
      coreutils \
      findutils \
      python3 \
      restic \
      rclone \
      tzdata \
    && addgroup -S -g 10001 webtui-backup \
    && adduser -S -D -H -u 10001 -G webtui-backup webtui-backup \
    && mkdir -p /var/lib/vpsttt-chat/backups /source/storage /source/config \
    && chown -R webtui-backup:webtui-backup /var/lib/vpsttt-chat/backups

COPY offsite_backup.py /usr/local/bin/offsite-backup

RUN chmod 0555 /usr/local/bin/offsite-backup

ENV HOME=/var/lib/vpsttt-chat/backups/home
ENV RESTIC_CACHE_DIR=/var/lib/vpsttt-chat/backups/restic-cache
ENV OFFSITE_BACKUP_WORK_ROOT=/var/lib/vpsttt-chat/backups/offsite
ENV SOURCE_STORAGE_PATH=/source/storage
ENV SOURCE_CONFIG_PATH=/source/config

USER 10001:10001

ENTRYPOINT ["python3", "/usr/local/bin/offsite-backup"]
CMD ["schedule"]
