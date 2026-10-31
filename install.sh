#!/usr/bin/env bash
# Downloads the release binary named by VERSION for this runner, verifies it
# against the release's checksums.txt, and writes its path to GITHUB_OUTPUT as
# bin. Every action in this repository runs it before running the binary.
set -euo pipefail
: "${VERSION:?}" "${OS:?}" "${ARCH:?}"

case "$OS" in
  Linux) os=linux ;;
  macOS) os=darwin ;;
  *) echo "::error::this action runs on Linux and macOS runners, not $OS"; exit 1 ;;
esac
case "$ARCH" in
  X64) arch=amd64 ;;
  ARM64) arch=arm64 ;;
  *) echo "::error::this action runs on X64 and ARM64 runners, not $ARCH"; exit 1 ;;
esac
asset="changelog_${os}_${arch}"

# Both forges carry every release. Each runner asks its own first, and the
# other covers one that is unreachable or has not received the release yet.
forgejo="https://forge.sarak.sh/actions/changelog/releases/download/v$VERSION"
github="https://github.com/mikluko/action-changelog/releases/download/v$VERSION"
if [ "$GITHUB_SERVER_URL" = https://github.com ]; then
  sources=("$github" "$forgejo")
else
  sources=("$forgejo" "$github")
fi

dir=$(mktemp -d "$RUNNER_TEMP/changelog.XXXXXX")
fetched=
for source in "${sources[@]}"; do
  if curl -fsSL --retry 3 -o "$dir/$asset" "$source/$asset" &&
    curl -fsSL --retry 3 -o "$dir/checksums.txt" "$source/checksums.txt"; then
    fetched=$source
    break
  fi
  echo "could not download $asset $VERSION from $source"
done
if [ -z "$fetched" ]; then
  echo "::error::could not download $asset $VERSION from any of ${sources[*]}"
  exit 1
fi

sum=$(awk -v name="$asset" '$2 == name' "$dir/checksums.txt")
if [ -z "$sum" ]; then
  echo "::error::$fetched/checksums.txt lists no $asset"
  exit 1
fi
if command -v sha256sum >/dev/null; then
  check=(sha256sum -c -)
else
  check=(shasum -a 256 -c -)
fi
if ! (cd "$dir" && printf '%s\n' "$sum" | "${check[@]}" >/dev/null); then
  echo "::error::$asset from $fetched does not match its checksum"
  exit 1
fi
chmod +x "$dir/$asset"
echo "bin=$dir/$asset" >>"$GITHUB_OUTPUT"
