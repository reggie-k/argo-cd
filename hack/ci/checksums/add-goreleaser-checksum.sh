#!/usr/bin/env sh

# Usage: ./add-goreleaser-checksum.sh v2.14.3
# The checksum is only relevat for linu

set -e

tarball=goreleaser_Linux_x86_64.tar.gz
out="$(git rev-parse --show-toplevel)/hack/ci/checksums/${tarball}.sha256"
wget -O- "https://github.com/goreleaser/goreleaser/releases/download/$1/checksums.txt" |
awk -v tarball="$tarball" '$NF==tarball{print;n=1;exit} END{exit !n}' >"$out"


 # checksumfile="helm-v$1-darwin-$arch.tar.gz.sha256"
 # wget "https://get.helm.sh/helm-v$1-darwin-$arch.tar.gz.sha256sum" -O "$checksumfile"
 # outname="$(git rev-parse --show-toplevel)/hack/installers/checksums/helm-v$1-darwin-$arch.tar.gz.sha256"
 # mv "$checksumfile" "$outname"