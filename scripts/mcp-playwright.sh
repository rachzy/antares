#!/usr/bin/env bash
# Launches Playwright MCP, reusing a preinstalled Chromium (cloud containers) or system Chrome when present.
set -euo pipefail

args=(--browser chromium --headless --isolated --no-sandbox)
for exe in /opt/pw-browsers/chromium /usr/bin/chromium /usr/bin/google-chrome-stable; do
  if [ -x "$exe" ]; then
    args+=(--executable-path "$exe")
    break
  fi
done

exec npx --yes @playwright/mcp@0.0.83 "${args[@]}"
