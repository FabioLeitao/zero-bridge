# Web UI check

Headless Chrome, Node 22 or newer, no npm packages. The bridge must already be listening.

```bash
ZB_FAKE_SCENARIO=rich ZB_FAKE_PERMISSION=1 \
  go run ./cmd/zero-bridge --listen 127.0.0.1:8769 --zero-bin /tmp/acpfake --permission-timeout 3s
node bridge/web/e2e/check.mjs http://127.0.0.1:8769/
```

The script loads the page, sends three prompts, and checks the hostile payload stays text, the permission card, Allow/Deny, expiry, the plan, and the diff. It then lists sessions (filter "todos"), checks a hostile title stays text, loads a replay, resumes without replay, and shows "sessao somente leitura" for the view-only session. With a token it also checks the device-session list, the empty and error states, "Sair de todos os dispositivos", and that the page does not overflow at 320px. It prints JSON and exits 1 on any failure. `--permission-timeout` has to be short enough for the expiry check, which waits 20s.

Portuguese: [README.pt_BR.md](README.pt_BR.md).
