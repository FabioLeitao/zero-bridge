# Container runtime

English. Portuguese: [CONTAINER.pt_BR.md](CONTAINER.pt_BR.md).

The bridge image listens on loopback inside the container. TLS stays on the proxy. See [CADDY.md](CADDY.md).

## Image

The runtime stage copies `zero-bridge` only. `acpfake` is a test double and is not in the image. The core module has no third-party requirement, so the image build does not copy a `go.sum`.

## Health

The image `HEALTHCHECK` runs:

```text
zero-bridge --healthcheck http://127.0.0.1:8765/health
```

That flag performs one GET and exits 0 only when the status is 200 and the body contains `"ok":true`. It does not follow redirects. It does not call `wget`.

## Confinement

`docker-compose.yml` sets these on the `zero-bridge` service and on the proxy service:

| Setting | Value |
| --- | --- |
| `read_only` | `true` |
| `cap_drop` | `ALL` |
| `security_opt` | `no-new-privileges:true` |

The Dockerfile does not apply them. Compose does. A tmpfs is mounted at `/tmp` because the root filesystem is read-only.

## Token

Mount the token file read-only. Do not bake the token into the image. The file mode on the host is `0600`.

## Telemetry

Export is off until `--otlp-endpoint` or `OTLP_ENDPOINT` is set. An `https` endpoint sends only the route, the method, the status, the ACP method, and an error flag. An `http` endpoint exports only when `OTEL_EXPORTER_OTLP_INSECURE=true`. The example Compose sets that variable because the local collector speaks HTTP.
