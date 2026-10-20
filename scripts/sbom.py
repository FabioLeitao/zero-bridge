#!/usr/bin/env python3
"""CycloneDX SBOM for the zero-bridge binary.

Uses the Go toolchain already on PATH. Does not install a package and does
not add a requirement to bridge/go.mod. The obs module is a separate binary
and is not linked into this one.
"""

import argparse
import json
import subprocess
import sys
import uuid
from pathlib import Path


def build(bridge: Path, dest: Path) -> None:
    subprocess.run(
        ["go", "build", "-o", str(dest), "./cmd/zero-bridge"],
        cwd=bridge,
        check=True,
    )


def version_info(binary: Path) -> dict:
    text = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
    info = {
        "go": "",
        "path": "",
        "mod": "",
        "version": "",
        "revision": "",
        "modified": "",
        "deps": [],
    }
    for line in text.splitlines():
        if "\t" not in line and ": go" in line:
            info["go"] = line.rsplit(": ", 1)[-1].split()[0]
            continue
        fields = line.split("\t")
        if len(fields) >= 3 and fields[1] == "path":
            info["path"] = fields[2]
        elif len(fields) >= 3 and fields[1] == "mod":
            info["mod"] = fields[2]
            info["version"] = fields[3] if len(fields) > 3 else ""
        elif len(fields) >= 4 and fields[1] == "dep":
            info["deps"].append({"name": fields[2], "version": fields[3]})
        elif len(fields) >= 3 and fields[1] == "build":
            key, _, val = fields[2].partition("=")
            if key == "vcs.revision":
                info["revision"] = val
            elif key == "vcs.modified":
                info["modified"] = val
    return info


def obs_module(obs: Path, module: str) -> dict | None:
    text = subprocess.check_output(
        ["go", "list", "-m", "-f", "{{.Path}} {{.Version}}", module],
        cwd=obs,
        text=True,
    ).strip()
    parts = text.split()
    if len(parts) != 2:
        return None
    return {"name": parts[0], "version": parts[1]}


def document(info: dict) -> dict:
    version = info["revision"] or info["version"] or "devel"
    if info["modified"] == "true":
        version += "+dirty"
    name = "zero-bridge"
    return {
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "serialNumber": "urn:uuid:" + str(uuid.uuid5(uuid.NAMESPACE_URL, name + "@" + version)),
        "version": 1,
        "metadata": {
            "component": {
                "type": "application",
                "name": name,
                "version": version,
            },
            "properties": [
                {"name": "go.module", "value": info["mod"] or info["path"]},
                {"name": "go.version", "value": info["go"]},
            ],
        },
        "components": [
            {"type": "library", "name": dep["name"], "version": dep["version"]}
            for dep in info["deps"]
        ],
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    bridge = args.root / "bridge"
    work = bridge / ".sbom-build"
    work.mkdir(mode=0o755, exist_ok=True)
    binary = work / "zero-bridge"
    build(bridge, binary)
    doc = document(version_info(binary))
    grpc = obs_module(bridge / "obs", "google.golang.org/grpc")
    if grpc is None:
        print("sbom missing obs grpc module", file=sys.stderr)
        return 1
    doc["components"].append(
        {
            "type": "library",
            "name": grpc["name"],
            "version": grpc["version"],
            "properties": [{"name": "zero-bridge.module", "value": "bridge/obs"}],
        }
    )
    raw = json.dumps(doc, indent=2) + "\n"
    if "/home/" in raw:
        print("sbom contained a home path", file=sys.stderr)
        return 1
    args.out.write_text(raw)
    binary.unlink(missing_ok=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
