#!/usr/bin/env bash

set -euo pipefail

hack_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

version="${TEMPORAL_CLI_VERSION:-1.5.0}"
base_url="https://github.com/temporalio/cli/releases/download/v${version}"
outdir="${TEMPORAL_CLI_OUTDIR:-${hack_dir}/temporal-cli}"

workdir=$(mktemp -d)
cleanup() {
  rm -rf "${workdir}"
}
trap cleanup EXIT

rm -rf "${outdir}"
mkdir -p "${outdir}"

for tuple in \
  "amd64 linux_amd64" \
  "arm64 linux_arm64"
do
  IFS=' ' read -r arch suffix <<< "${tuple}"
  tmp="${workdir}/${suffix}"
  mkdir -p "${tmp}"

  archive="temporal_cli_${version}_${suffix}.tar.gz"
  url="${base_url}/${archive}"

  echo "Fetching ${url}"
  curl -fsSL "${url}" | tar -xz -C "${tmp}"

  mkdir -p "${outdir}/linux-${arch}"
  mv "${tmp}/temporal" "${outdir}/linux-${arch}/temporal"
  chmod 755 "${outdir}/linux-${arch}/temporal"

  if [[ ! -f ${outdir}/LICENSE ]]; then
    install -m 0644 "${tmp}/LICENSE" "${outdir}/LICENSE"
  fi
done
