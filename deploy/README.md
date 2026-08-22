# Deployment

The reference deployment uses `docker-compose.yml` for an application and PostgreSQL pair. Production operators should provide secrets through their orchestrator, terminate TLS at a trusted reverse proxy, persist `/app/var/files`, and schedule PostgreSQL backups independently from the application container.

The image runs as UID 10001, exposes port 8080, supports amd64 and arm64, and applies the idempotent schema migration before starting the service.
