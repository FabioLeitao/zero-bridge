# Revisão de vulnerabilidade

Português. Canônico: [VULN_REVIEW.md](VULN_REVIEW.md).

Data da revisão: 2026-10-20. Scanner: `govulncheck` v1.3.0, em `bridge/obs`, com `-show verbose`.

## GO-2026-6443

| Campo | Valor |
| --- | --- |
| ID | GO-2026-6443 |
| Pacote | `google.golang.org/grpc` |
| Encontrado em | v1.83.1 |
| Corrigido em | v1.83.2 |
| Resumo | Pânico do servidor quando falta o authority ou o cabeçalho Host |
| Chamado nesta árvore | Não |

O `govulncheck` lista o módulo nos resultados de pacote e não lista símbolo nenhum. A frase da varredura é que o código não parece chamar a vulnerabilidade.

`bridge/obs` monta o exportador HTTP de traces e o exportador HTTP de métricas do OpenTelemetry. Esse grafo importa `google.golang.org/grpc`. O sidecar não chama `grpc.NewServer`, não serve gRPC, e o exportador de log é no-op. O pânico está no caminho do servidor gRPC. Este processo não abre esse caminho.

O módulo do núcleo, `bridge/`, não exige `google.golang.org/grpc`. `bridge/go.mod` continua na biblioteca padrão.

Esta revisão não sobe o módulo. A correção publicada é a v1.83.2. Um bump posterior pode pegá-la. Até lá o achado continua um pacote importado e não chamado, não um servidor alcançável nesta árvore.
