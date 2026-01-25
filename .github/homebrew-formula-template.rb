# Homebrew formula template for Tap
# This file is automatically updated by the release workflow.
# Manual edits will be overwritten on the next release.

class Tap < Formula
  desc "Markdown presentations with live code execution"
  homepage "https://tap.sh"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/MiniCodeMonkey/tap/releases/download/vVERSION_PLACEHOLDER/tap-darwin-arm64"
      sha256 "SHA256_DARWIN_ARM64_PLACEHOLDER"
    else
      url "https://github.com/MiniCodeMonkey/tap/releases/download/vVERSION_PLACEHOLDER/tap-darwin-amd64"
      sha256 "SHA256_DARWIN_AMD64_PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/MiniCodeMonkey/tap/releases/download/vVERSION_PLACEHOLDER/tap-linux-arm64"
      sha256 "SHA256_LINUX_ARM64_PLACEHOLDER"
    else
      url "https://github.com/MiniCodeMonkey/tap/releases/download/vVERSION_PLACEHOLDER/tap-linux-amd64"
      sha256 "SHA256_LINUX_AMD64_PLACEHOLDER"
    end
  end

  def install
    if OS.mac? && Hardware::CPU.arm?
      bin.install "tap-darwin-arm64" => "tap"
    elsif OS.mac?
      bin.install "tap-darwin-amd64" => "tap"
    elsif OS.linux? && Hardware::CPU.arm?
      bin.install "tap-linux-arm64" => "tap"
    else
      bin.install "tap-linux-amd64" => "tap"
    end
  end

  test do
    system "#{bin}/tap", "--version"
  end
end
