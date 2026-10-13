# zero-bridge

Ponte local em torno do [Zero](https://github.com/Twigpine/zero): um programa Go pequeno que pode expor `zero acp` via HTTP/SSE e, opcionalmente, A2A de entrada, além de plugin tools do Zero para A2A de saída. O Zero continua sendo o agente; este repositório não faz fork nem altera o core daqui.

**Nada está implementado ainda.** Não há `go.mod`, servidor nem esqueleto de plugin nesta árvore.

## Plano

Fatias incrementais (trilhas A/B/C) em [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md) (inglês, canônico). Português: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

## Licença

Ainda não definida. O [zero-handoff](https://github.com/FabioLeitao/zero-handoff) não traz arquivo `LICENSE`; este repo segue a mesma postura até o operador escolher.

## Trabalho relacionado (fora do escopo aqui)

- Correções de ACP e PRs no upstream: branches de tópico no fork do Zero, não neste repo.
- Handoff de sessão sem socket: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
