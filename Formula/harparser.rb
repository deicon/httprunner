class Harparser < Formula
  desc "HAR to .http extractor"
  homepage "https://github.com/deicon/httprunner"
  version "0.0.0" # updated by CI on release

  # URLs and sha256 are set per-platform by CI on release
  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/v0.0.0/harparser_v0.0.0_darwin_arm64.tar.gz"
      sha256 "deadbeef"
    else
      url "https://github.com/deicon/httprunner/releases/download/v0.0.0/harparser_v0.0.0_darwin_amd64.tar.gz"
      sha256 "deadbeef"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/v0.0.0/harparser_v0.0.0_linux_arm64.tar.gz"
      sha256 "deadbeef"
    else
      url "https://github.com/deicon/httprunner/releases/download/v0.0.0/harparser_v0.0.0_linux_amd64.tar.gz"
      sha256 "deadbeef"
    end
  end

  def install
    bin.install "harparser"
  end

  test do
    system "#{bin}/harparser", "-version"
  end
end

