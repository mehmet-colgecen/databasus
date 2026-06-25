# SSL test certificates

Self-signed cert + key used by the Postgres SSL test container.
Mounted into the SSL container defined in `docker-compose.yml`. Not used in production.

The cert validates against `CN=localhost`, but every backup/restore code path uses
`InsecureSkipVerify`/`--skip-ssl-verify-server-cert`, so any self-signed cert works.

To regenerate (run from this directory):

```
openssl req -x509 -newkey rsa:2048 -keyout server.key -out server.crt -days 3650 -nodes -subj "/CN=localhost"
cat server.crt server.key > server.pem
```

- `server.crt` / `server.key` — used by Postgres
- `server.pem` — combined cert+key
- `pg_hba.conf` — SSL-only auth rules for the Postgres SSL container; rejects every plaintext TCP connection so a silent SSL-drop regression fails the test
