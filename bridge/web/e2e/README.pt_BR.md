# Verificação da UI web

Chrome headless, Node 22 ou mais novo, sem pacotes npm. A ponte já precisa estar escutando.

```bash
ZB_FAKE_SCENARIO=rich ZB_FAKE_PERMISSION=1 \
  go run ./cmd/zero-bridge --listen 127.0.0.1:8769 --zero-bin /tmp/acpfake --permission-timeout 3s
node bridge/web/e2e/check.mjs http://127.0.0.1:8769/
```

O script abre a página, envia três prompts e confere que o payload hostil continua texto, o cartão de permissão, Allow/Deny, a expiração, o plano e o diff. Depois lista sessões (filtro "todos"), confere que um título hostil continua texto, carrega um replay, retoma sem replay e mostra "sessao somente leitura" na sessão só de leitura. Com token, também confere a lista de sessões do acesso, os estados vazio e de erro, "Sair de todos os dispositivos", e que a página não estoura em 320px. Imprime JSON e sai 1 se algo falhar. `--permission-timeout` precisa ser curto o bastante para a checagem de expiração, que espera 20s.

English: [README.md](README.md).
