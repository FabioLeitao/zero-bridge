# Agentes

Português (espelho). Canônico em inglês: [AGENTS.md](AGENTS.md).

Estas regras valem para todo agente que edita este repositório. Quem contribui como pessoa segue [CONTRIBUTING.pt_BR.md](CONTRIBUTING.pt_BR.md). O plano das fatias está em [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

## Papéis

O agente edita a árvore. O auditor revisa o commit. O operador decide escopo, push e visibilidade. Não faça push, não abra pull request e não mude a visibilidade do repositório, a menos que o operador diga isso nesta conversa.

## Regras duras

- O módulo do core `bridge/` fica na biblioteca padrão do Go. Nenhuma dependência nova em `bridge/go.mod`. A única exceção planejada é o submódulo `bridge/obs`, que pode importar o SDK de OpenTelemetry e ainda não está construído.
- A UI web insere texto só com `textContent` e `createTextNode`.
- O processo escuta em TCP de loopback, ou num socket Unix em `unix:<caminho absoluto>` (pai `0700`, socket `0600`). Ele não termina TLS. Named pipe do Windows não está implementado. No socket, um Origin só é aceito com `--public-origin`.
- Log, telemetria, URL, query string e eventos enviados pelo servidor não carregam segredo, prompt, `rawInput`, id de sessão nem caminho de diretório pessoal.
- O rótulo de uma sessão é `working`, `blocked`, `done`, `error` ou `idle`, derivado do tráfego ACP que a ponte já vê. O rótulo não carrega prompt, argumento de ferramenta nem título. O perfil `generic` informa só `working` e `idle`.
- Não mate processo por padrão no nome. Encerre um processo que você mesmo subiu pelo pid registrado, depois de ler a linha de comando.
- Não pule um gancho do git. Testes que criam arquivos rodam com umask `022`.
- Ferramentas externas rodam como `go run pkg@version` pinado. Nada é instalado globalmente para o gate.

## Validação

`make check` é obrigatório antes de um commit. Antes de uma pull request, rode também `make e2e` e `make mutate`. `make mutate` é a camada lenta de pré-PR. Não é o gancho de pre-commit.

Enquanto o commit ainda está sendo escrito, rode `go test -run` nos pacotes que ele toca. Rode `make check` uma vez, depois disso, antes do commit.

Um clone não instala os ganchos. Rode `make hooks` uma vez.

## Um commit

Um commit é uma coisa. Compila, os testes que o cobrem passam, e ele pode ser revertido sozinho. Não faça amend, reset nem rebase, a menos que o operador peça essa operação exata. Assine o commit. Assinatura ausente ou ruim falha no gate; uma assinatura que este clone não consegue verificar é um aviso, a menos que um arquivo de signatários permitidos esteja configurado. Não acrescente trailer de coautor. Coloque arquivos no stage pelo nome.

## Fluxo da fatia

O trabalho cai numa branch `epic/sN-slug`. O corpo da pull request nomeia a issue com uma palavra que fecha issue. A mensagem do commit não nomeia. O merge é `git merge --merge`, para os commits originais permanecerem. Squash merge e rebase merge são recusados.

## Habilidades disponíveis

As habilidades disponíveis no sistema são as mesmas da versão em inglês. Consulte [AGENTS.md](AGENTS.md) para a lista completa.
