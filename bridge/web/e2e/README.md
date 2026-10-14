# Web UI check

Headless Chrome, Node 22 or newer, no npm packages. The bridge must already be listening.

```bash
ZB_FAKE_SCENARIO=rich ZB_FAKE_PERMISSION=1 \
  go run ./cmd/zero-bridge --listen 127.0.0.1:8769 --zero-bin /tmp/acpfake --permission-timeout 3s
node bridge/web/e2e/check.mjs http://127.0.0.1:8769/
```

The script loads the page, sends three prompts, and checks the hostile payload stays text, the permission card, Allow/Deny, expiry, the plan, and the diff. It prints JSON and exits 1 on any failure. `--permission-timeout` has to be short enough for the expiry check, which waits 20s.
