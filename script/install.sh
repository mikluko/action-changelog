#!/usr/bin/env bash
#
# Downloads the action-changelog release matching this action's ref and puts it
# on PATH for the steps that follow.
#
# The version is resolved rather than committed into the download URL: a
# workflow naming a tag in `uses:` gets that tag, and a workflow pinning a SHA
# gets the VERSION file committed at that SHA, which names the release the
# commit belongs to. Nothing here writes back to the repository.

set -euo pipefail

readonly REPO=mikluko/action-changelog

version="${INPUT_VERSION:-}"
if [[ -z $version ]]; then
    # github.action_ref is the tag when `uses:` names one and the SHA when it
    # pins one, and only the former is a release.
    if [[ ${ACTION_REF:-} =~ ^v[0-9] ]]; then
        version=$ACTION_REF
    else
        version=$(tr -d '[:space:]' <"$ACTION_PATH/VERSION")
    fi
fi

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    MINGW* | MSYS* | CYGWIN*) os=windows ;;
    *)
        echo "action-changelog: unsupported operating system $(uname -s)" >&2
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *)
        echo "action-changelog: unsupported architecture $(uname -m)" >&2
        exit 1
        ;;
esac

readonly archive="action-changelog_${os}_${arch}.tar.gz"
readonly base="https://github.com/$REPO/releases/download/$version"
readonly dir="$RUNNER_TEMP/action-changelog"

mkdir -p "$dir"
cd "$dir"

curl --fail --silent --show-error --location --retry 3 \
    --remote-name "$base/$archive" \
    --remote-name "$base/checksums.txt"

# A release asset is served from a CDN and the ref that named it may be a
# mutable major tag, so the download is verified rather than trusted.
grep " $archive\$" checksums.txt >expected.txt
if command -v sha256sum >/dev/null; then
    sha256sum --check --strict expected.txt
else
    # macOS ships shasum and not sha256sum.
    shasum --algorithm 256 --check --strict expected.txt
fi

tar --extract --file "$archive"
echo "$dir" >>"$GITHUB_PATH"
