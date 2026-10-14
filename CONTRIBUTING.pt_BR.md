# Como contribuir

Português (espelho). Canônico em inglês: [CONTRIBUTING.md](CONTRIBUTING.md).

A ponte está em desenvolvimento controlado. Relatos de bug e ideias revisadas são bem-vindos. Leia isto antes de abrir uma pull request.

## Issue primeiro

Procure as issues abertas antes de criar outra.

- Um bug é uma issue, um defeito por issue, com reprodução: versão, comando, o que você esperava e o que aconteceu. Logs não levam token, prompt nem caminho de diretório pessoal.
- Uma ideia começa numa Discussion do GitHub quando Discussions estiver ligado. Use o modelo de ideia quando um mantenedor pedir uma issue. Não abra pull request a partir de uma ideia que ainda não foi aceita.
- Relatos e patches assistidos por IA são bem-vindos quando o autor verificou cada um. Não publique um lote sem verificação.

## Quantos por vez

Mantenha poucas issues e pull requests abertas ao mesmo tempo. Termine as que já estão em revisão antes de abrir mais. Mantenedores podem pedir para dividir ou segurar um envio.

## Pull request

Aponte a issue pai no corpo da pull request. Diga o que mudou e por quê. Cole a saída de `make check`. Antes da pull request rode também `make e2e` e `make mutate`, e cole essas saídas. Nomeie os riscos.

Um clone não instala os ganchos. Rode `make hooks` uma vez e depois `make check`, antes da pull request. Pular um gancho é proibido.

Fique dentro da issue. Não misture uma correção alheia, uma varrida de formatação ou uma dependência nova na mesma pull request.

## Validação

Na raiz do repositório:

1. `make hooks` uma vez por clone.
2. `make check` antes de cada commit e de novo antes da pull request.
3. `make e2e` e `make mutate` antes da pull request. `make mutate` é lento e não faz parte do gancho de pre-commit.

`make check` é a definição de pronto. Um CI futuro chama esse alvo e não cria uma segunda lista.

## Commits e merge

Assine cada commit. Não acrescente trailer de coautor. A mensagem diz por que a mudança existe.

`Refs` e uma palavra do GitHub que fecha issue ficam no corpo da pull request, nunca na mensagem do commit.

Faça merge com um merge commit para os commits originais permanecerem na branch. Squash merge e rebase merge são recusados.

## Documentação

A documentação para quem usa o projeto existe em inglês e em português (`*.md` e `*.pt_BR.md`). Os dois arquivos têm o mesmo número de títulos `##` e o mesmo número de linhas de tabela. Mudou um, atualize o outro no mesmo commit.

## Relatos de segurança

Não abra issue pública para uma vulnerabilidade. Abra um security advisory privado neste repositório e espere um mantenedor.
