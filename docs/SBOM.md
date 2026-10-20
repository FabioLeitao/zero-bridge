# SBOM

English. Portuguese: [SBOM.pt_BR.md](SBOM.pt_BR.md).

`make sbom` writes `bridge/zero-bridge.cdx.json`, a CycloneDX 1.5 document for the `zero-bridge` binary. The script is `scripts/sbom.py`. It uses `go build` and `go version -m`. It does not install a tool and it does not add a requirement to `bridge/go.mod`.

The core binary has no third-party module. `bridge/obs` is a separate module and is not linked into this binary. `govulncheck` covers both modules.

The JSON names the module, the Go version, and the VCS revision. It does not name a home directory.
