Pre-built DB client binaries committed to the repo so that local dev,
CI, and the Docker image all read from one place. The Go backend
resolves them at runtime via `runtime.GOOS`+`runtime.GOARCH` →
`assets/tools/<arch-key>/<db>/<db>-<v>/bin/<command>[.exe]`.

Layout (one subtree per arch, identical shape):

```
assets/tools/<arch>/
  postgresql/postgresql-{12,13,14,15,16,17,18}/bin/
    pg_dump, pg_restore, psql   (+ minimal *.dll set on Windows)
```

`<arch>` keys (mapping in `backend/internal/util/tools/paths.go`):

| GOOS / GOARCH       | key       | size  |
|---------------------|-----------|-------|
| `linux` / `amd64`   | `x64`     | ~160 MB |
| `linux` / `arm64`   | `arm`     | ~125 MB |
| `windows` / `amd64` | `win-x64` | ~120 MB |

To refresh a tool set, drop the corresponding `bin/` contents in place
and commit. There are no install scripts in this directory; binaries
are sourced from the upstream vendor downloads.
