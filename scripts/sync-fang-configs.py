"""Rewrite the config choice list of the train-model workflow from `fang configs`."""

import re
import subprocess
import sys
from pathlib import Path

WORKFLOW = Path(".github/workflows/train-model.yml")
BLOCK = re.compile(r"(      config:\n(?:        .*\n)*?        options:\n)((?:          - .*\n)+)")


def main() -> int:
    names = subprocess.run(
        ["uv", "run", "--project", "apps/fang", "fang", "configs"],
        check=True, capture_output=True, text=True,
    ).stdout.split()
    text = WORKFLOW.read_text()
    updated = BLOCK.sub(lambda m: m.group(1) + "".join(f"          - {n}\n" for n in names), text, 1)
    if "--check" in sys.argv:
        if updated != text:
            print("train-model.yml config options are stale; run `make sync-configs`.")
            return 1
        return 0
    WORKFLOW.write_text(updated)
    return 0


if __name__ == "__main__":
    sys.exit(main())
