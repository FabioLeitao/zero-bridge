# Runtime do container

Português. Canônico em inglês: [CONTAINER.md](CONTAINER.md).

A imagem da ponte escuta em loopback dentro do container. O TLS fica no proxy. Veja [CADDY.pt_BR.md](CADDY.pt_BR.md).

## Imagem

O estágio final copia só `zero-bridge`. `acpfake` é um dublê de teste e não entra na imagem. O módulo do núcleo não tem dependência de terceiro, então o build da imagem não copia um `go.sum`.

## Saúde

O `HEALTHCHECK` da imagem roda:

```text
zero-bridge --healthcheck http://127.0.0.1:8765/health
```

Essa flag faz um GET e sai 0 só quando o status é 200 e o corpo contém `"ok":true`. Ela não segue redirect. Ela não chama `wget`.

## Confinamento

O `docker-compose.yml` liga isto no serviço `zero-bridge` e no serviço do proxy:

| Ajuste | Valor |
| --- | --- |
| `read_only` | `true` |
| `cap_drop` | `ALL` |
| `security_opt` | `no-new-privileges:true` |

O Dockerfile não aplica isso. O Compose aplica. Um tmpfs fica em `/tmp` porque o sistema de arquivos raiz é somente leitura.

## Token

Monte o arquivo de token somente leitura em `/var/lib/zerobridge/.local/state/zero-bridge/token.txt`. Não grave o token na imagem. O modo do arquivo no host é `0600`. O comando da imagem e o `Exec` do Quadlet passam `--token-file` com esse caminho e `--require-token`. O Compose e o pod kube não têm comando próprio, então usam o comando da imagem. `ZERO_BRIDGE_TOKEN_FILE` não é lido. Sem token resolvido, `--require-token` faz o processo sair **2** antes de escutar.

## Telemetria

O export sai desligado. Com `--otlp-endpoint` ou `OTLP_ENDPOINT` em `https`, a ponte envia só rota, método, status, método ACP e um flag de erro. Um endpoint `http` só exporta quando `OTEL_EXPORTER_OTLP_INSECURE=true`. O Compose de exemplo liga essa variável porque o collector local fala HTTP.
