#!/usr/bin/env bash
# Test the multiline GITHUB_OUTPUT write used after GoReleaser in .github/workflows/release.yaml
# without running GoReleaser (~1h). Run from repo root:
#   ./hack/ci/test-goreleaser-github-output.sh
#   ./hack/ci/test-goreleaser-github-output.sh path/to/artifacts.json
# Same check in a container (needs Docker): act workflow_dispatch -W .github/workflows/goreleaser-github-output-smoke.yaml -j smoke
set -euo pipefail

tmpdir=""
github_output_tmp=""
cleanup() {
	rm -f "${github_output_tmp:-}"
	[[ -n "${tmpdir:-}" ]] && rm -rf "$tmpdir"
}
trap cleanup EXIT

json_file=${1:-}
if [[ -z "$json_file" ]]; then
	tmpdir=$(mktemp -d)
	json_file="$tmpdir/artifacts.json"
	# Minimal array like GoReleaser; includes substring that used to be the fixed delimiter (safe when embedded).
	cat >"$json_file" <<'JSON'
[{"name":"cli_checksums.txt","type":"Checksum","path":"dist/checksums.txt"}]
JSON
fi

if ! test -f "$json_file"; then
	echo "File not found: $json_file" >&2
	exit 1
fi

github_output_tmp=$(mktemp)
GITHUB_OUTPUT=$github_output_tmp

# Keep in sync with release.yaml "Run GoReleaser" step (output write only).
delim="GORELEASER_ARTIFACTS_JSON_$(openssl rand -hex 16)"
{
	echo "artifacts<<${delim}"
	cat "$json_file"
	echo "${delim}"
} >>"$GITHUB_OUTPUT"

python3 - "$GITHUB_OUTPUT" <<'PY'
import json, sys
path = sys.argv[1]
data = open(path, encoding="utf-8").read().splitlines()
out = {}
i = 0
while i < len(data):
    line = data[i]
    if "<<" in line and not line.lstrip().startswith("#"):
        name, _, delim = line.partition("<<")
        delim = delim.strip()
        if not delim:
            raise SystemExit(f"bad line: {line!r}")
        i += 1
        chunk = []
        while i < len(data) and data[i] != delim:
            chunk.append(data[i])
            i += 1
        if i >= len(data) or data[i] != delim:
            raise SystemExit(f"missing closing delimiter {delim!r} (same bug as CI)")
        out[name] = "\n".join(chunk)
        i += 1
        continue
    if "=" in line and "<<" not in line:
        k, _, v = line.partition("=")
        out[k] = v
        i += 1
        continue
    i += 1

artifacts = out.get("artifacts")
if not artifacts:
    raise SystemExit("no 'artifacts' output found")
json.loads(artifacts)
print("ok: GITHUB_OUTPUT multiline block closed correctly and artifacts is valid JSON")
PY

echo "checked file: $json_file"
