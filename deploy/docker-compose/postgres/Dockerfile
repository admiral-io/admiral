FROM postgres:18

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        postgresql-18-pgaudit \
        postgresql-18-cron \
        postgresql-18-partman \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/
