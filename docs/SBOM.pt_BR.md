# SBOM

Português. Canônico em inglês: [SBOM.md](SBOM.md).

`make sbom` grava `bridge/zero-bridge.cdx.json`, um documento CycloneDX 1.5 do binário `zero-bridge`. O script é `scripts/sbom.py`. Ele usa `go build` e `go version -m`. Não instala ferramenta e não acrescenta requisito em `bridge/go.mod`.

O binário do núcleo não tem módulo de terceiro. `bridge/obs` é um módulo separado e não entra nesse binário. O documento ainda nomeia `google.golang.org/grpc` desse módulo, com a propriedade `zero-bridge.module` em `bridge/obs`. O `govulncheck` cobre os dois módulos.

O JSON nomeia o módulo, a versão do Go e a revisão VCS. Não nomeia um diretório home.
