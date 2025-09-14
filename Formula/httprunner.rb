class Httprunner < Formula
  desc "Parallel HTTP scenario runner"
  homepage "https://github.com/deicon/httprunner"
  version "1.3.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/httprunner_1.3.0_darwin_arm64.tar.gz"
      sha256 "615ed82f9db67c806f2e4706fa603ed79489a80d2c48d52817ef495b27bb82f1"
    else
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/httprunner_1.3.0_darwin_amd64.tar.gz"
      sha256 "de39dd6a0ba2f73d473dfcde3ac62fb93e0c54c2b8c1075b754c228d9c96b445"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/httprunner_1.3.0_linux_arm64.tar.gz"
      sha256 "1dc0e3143f03b818dd701d46069e2ae766f51a243dca7df2d4b0f082c37b2b7c"
    else
      url "https://github.com/deicon/httprunner/releases/download/1.3.0/httprunner_1.3.0_linux_amd64.tar.gz"
      sha256 "34a87a804991b54030b01c8f58930f2472a00d2c5cccb7ee64a8174142342787"
    end
  end

  def install
    bin.install "httprunner"
  end

  test do
    system "#{bin}/httprunner", "-version"
  end
end
