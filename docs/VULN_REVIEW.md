# Vulnerability review

English (canonical). Portuguese: [VULN_REVIEW.pt_BR.md](VULN_REVIEW.pt_BR.md).

Review date: 2026-10-20. Scanner: `govulncheck` v1.3.0, `bridge/obs`, `-show verbose`.

## GO-2026-6443

| Field | Value |
| --- | --- |
| ID | GO-2026-6443 |
| Package | `google.golang.org/grpc` |
| Found in | v1.83.1, before this bump |
| Fixed in | v1.83.2 |
| Selected now | v1.83.2 |
| Summary | Server panic when the authority or the Host header is missing |
| Called from this tree | No |

`govulncheck` v1.3.0 on `bridge/obs` after the bump reports no vulnerabilities. The earlier scan, on v1.83.1, listed the module under package results and listed no symbol results.

`bridge/obs` builds the OpenTelemetry HTTP trace exporter and the HTTP metric exporter. That module graph imports `google.golang.org/grpc`. The sidecar does not call `grpc.NewServer`, does not serve gRPC, and its log exporter is a no-op. The panic is on the gRPC server path. This process never opens that path.

The core module `bridge/` does not require `google.golang.org/grpc`. `bridge/go.mod` stays on the standard library.

This review bumps `bridge/obs` from v1.83.1 to v1.83.2. No other module in `bridge/obs/go.mod` moved. `make sbom` names `google.golang.org/grpc` v1.83.2 with `zero-bridge.module` set to `bridge/obs`. The core binary still has no third-party component.
