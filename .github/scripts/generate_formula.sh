#!/usr/bin/env bash
set -euo pipefail

# Inputs
REPO="${GITHUB_REPOSITORY:-deicon/httprunner}"
TAG="${1:?usage: generate_formula.sh <tag> (e.g., v1.2.3)}"
VERSION_STRIPPED="${TAG#v}"
BUILD_DIR="build"

base_url="https://github.com/${REPO}/releases/download/${TAG}"

sha256_for() {
  local file="$1"
  shasum -a 256 "$file" | awk '{print $1}'
}

write_formula() {
  local tool="$1"

  local darwin_arm64_file="${tool}_${TAG}_darwin_arm64.tar.gz"
  local darwin_amd64_file="${tool}_${TAG}_darwin_amd64.tar.gz"
  local linux_arm64_file="${tool}_${TAG}_linux_arm64.tar.gz"
  local linux_amd64_file="${tool}_${TAG}_linux_amd64.tar.gz"

  local darwin_arm64_sha linux_arm64_sha darwin_amd64_sha linux_amd64_sha
  darwin_arm64_sha=$(sha256_for "${BUILD_DIR}/${darwin_arm64_file}")
  darwin_amd64_sha=$(sha256_for "${BUILD_DIR}/${darwin_amd64_file}")
  linux_arm64_sha=$(sha256_for "${BUILD_DIR}/${linux_arm64_file}")
  linux_amd64_sha=$(sha256_for "${BUILD_DIR}/${linux_amd64_file}")

  local class_name
  case "$tool" in
    httprunner) class_name="Httprunner" ;;
    harparser) class_name="Harparser" ;;
    *) echo "Unknown tool: $tool" >&2; exit 1 ;;
  esac

  cat > "Formula/${tool}.rb" <<EOF
class ${class_name} < Formula
  desc "$( [ "$tool" = "httprunner" ] && echo "Parallel HTTP scenario runner" || echo "HAR to .http extractor" )"
  homepage "https://github.com/deicon/httprunner"
  version "${VERSION_STRIPPED}"

  on_macos do
    if Hardware::CPU.arm?
      url "${base_url}/${darwin_arm64_file}"
      sha256 "${darwin_arm64_sha}"
    else
      url "${base_url}/${darwin_amd64_file}"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${base_url}/${linux_arm64_file}"
      sha256 "${linux_arm64_sha}"
    else
      url "${base_url}/${linux_amd64_file}"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "${tool}"
  end

  test do
    system "#{bin}/${tool}", "-version"
  end
end
EOF
}

write_formula httprunner
write_formula harparser

echo "Updated Formula/httprunner.rb and Formula/harparser.rb for ${TAG}"
