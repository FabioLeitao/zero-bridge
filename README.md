# zero-bridge

Local bridge around [Zero](https://github.com/Twigpine/zero): a small Go program that can expose `zero acp` over HTTP/SSE and optional A2A ingress, plus Zero plugin tools for outbound A2A. Zero itself stays the agent; this repo does not fork or patch the core from here.

**Nothing is implemented yet.** There is no `go.mod`, no server, and no plugin skeleton in this tree.

## Plan

Incremental slices (tracks A/B/C) live under [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md) (English, canonical). Portuguese: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

## License

Not chosen yet. [zero-handoff](https://github.com/FabioLeitao/zero-handoff) ships without a `LICENSE` file; this repo follows the same “decide later” posture until the operator picks one.

## Related work (out of scope here)

- ACP fixes and upstream PRs: topic branches on the Zero fork, not this repo.
- Session handoff without sockets: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
