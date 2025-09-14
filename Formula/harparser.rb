class Harparser < Formula
  desc "HAR to .http extractor"
  homepage "https://github.com/deicon/httprunner"
  version "1.3.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/harparser_1.3.0_darwin_arm64.tar.gz"
      sha256 "ffe25f3e3f9d845edef363bfdc514f37bb434a40701f8b9099e794e503d6d243"
    else
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/harparser_1.3.0_darwin_amd64.tar.gz"
      sha256 "91404c91a37969cb88a3ad4b098d3ed7a19b8e36eb8d177bf73c2a67ba88d0a1"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/harparser_1.3.0_linux_arm64.tar.gz"
      sha256 "0f2748f710d12f01cdb8459ad376942ed2b97f581ad7a6e60640d18465dbac88"
    else
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/harparser_1.3.0_linux_amd64.tar.gz"
      sha256 "042caeaf8dc8ed5aa946d5b074984af20ccc21ffdbca10af637a1450aef97d7e"
    end
  end

  def install
    bin.install "harparser"
  end

  test do
    system "#{bin}/harparser", "-version"
  end
end
