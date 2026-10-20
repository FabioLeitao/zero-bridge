# Revisão de vulnerabilidade

Português. Canônico: [VULN_REVIEW.md](VULN_REVIEW.md).

Data da revisão: 2026-10-20. Scanner: `govulncheck` v1.3.0, em `bridge/obs`, com `-show verbose`.

## GO-2026-6443

| Campo | Valor |
| --- | --- |
| ID | GO-2026-6443 |
| Pacote | `google.golang.org/grpc` |
| Encontrado em | v1.83.1, antes deste bump |
| Corrigido em | v1.83.2 |
| Selecionado agora | v1.83.2 |
| Resumo | Pânico do servidor quando falta o authority ou o cabeçalho Host |
| Chamado nesta árvore | Não |

O `govulncheck` v1.3.0 em `bridge/obs` depois do bump não reporta vulnerabilidade. A varredura anterior, na v1.83.1, listava o módulo nos resultados de pacote e não listava símbolo nenhum.

`bridge/obs` monta o exportador HTTP de traces e o exportador HTTP de métricas do OpenTelemetry. Esse grafo importa `google.golang.org/grpc`. O sidecar não chama `grpc.NewServer`, não serve gRPC, e o exportador de log é no-op. O pânico está no caminho do servidor gRPC. Este processo não abre esse caminho.

O módulo do núcleo, `bridge/`, não exige `google.golang.org/grpc`. `bridge/go.mod` continua na biblioteca padrão.

Esta revisão sobe `bridge/obs` de v1.83.1 para v1.83.2. Nenhum outro módulo em `bridge/obs/go.mod` mudou. O `make sbom` nomeia `google.golang.org/grpc` v1.83.2 com `zero-bridge.module` em `bridge/obs`. O binário do núcleo continua sem componente de terceiro.
