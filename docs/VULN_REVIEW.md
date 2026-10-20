# Vulnerability review

English (canonical). Portuguese: [VULN_REVIEW.pt_BR.md](VULN_REVIEW.pt_BR.md).

Review date: 2026-10-20. Scanner: `govulncheck` v1.3.0, `bridge/obs`, `-show verbose`.

## GO-2026-6443

| Field | Value |
| --- | --- |
| ID | GO-2026-6443 |
| Package | `google.golang.org/grpc` |
| Found in | v1.83.1 |
| Fixed in | v1.83.2 |
| Summary | Server panic when the authority or the Host header is missing |
| Called from this tree | No |

`govulncheck` reports the module under package results and reports no symbol results. The sentence in the scan is that the code does not appear to call the vulnerability.

`bridge/obs` builds the OpenTelemetry HTTP trace exporter and the HTTP metric exporter. That module graph imports `google.golang.org/grpc`. The sidecar does not call `grpc.NewServer`, does not serve gRPC, and its log exporter is a no-op. The panic is on the gRPC server path. This process never opens that path.

The core module `bridge/` does not require `google.golang.org/grpc`. `bridge/go.mod` stays on the standard library.

This review does not bump the module. The fix release is v1.83.2. A later dependency bump can take it. Until then the finding stays an imported, uncalled package, not a reachable server in this tree.
