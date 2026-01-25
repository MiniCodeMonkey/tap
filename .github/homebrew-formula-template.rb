# Homebrew formula template for Tap
# Copy this to your homebrew-tap repo as Formula/tap.rb
#
# Setup:
# 1. Create repo: github.com/YOUR_ORG/homebrew-tap
# 2. Copy this file to: Formula/tap.rb
# 3. Users install with: brew install YOUR_ORG/tap/tap

class Tap < Formula
  desc "Markdown presentations with live code execution"
  homepage "https://tap.sh"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/tap-slides/tap/releases/download/vVERSION_PLACEHOLDER/tap-darwin-arm64"
      sha256 "SHA256_DARWIN_ARM64_PLACEHOLDER"
    else
      url "https://github.com/tap-slides/tap/releases/download/vVERSION_PLACEHOLDER/tap-darwin-amd64"
      sha256 "SHA256_DARWIN_AMD64_PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/tap-slides/tap/releases/download/vVERSION_PLACEHOLDER/tap-linux-arm64"
      sha256 "SHA256_LINUX_ARM64_PLACEHOLDER"
    else
      url "https://github.com/tap-slides/tap/releases/download/vVERSION_PLACEHOLDER/tap-linux-amd64"
      sha256 "SHA256_LINUX_AMD64_PLACEHOLDER"
    end
  end

  def install
    binary_name = "tap-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    binary_name = "tap-darwin-arm64" if OS.mac? && Hardware::CPU.arm?
    binary_name = "tap-darwin-amd64" if OS.mac? && !Hardware::CPU.arm?
    binary_name = "tap-linux-arm64" if OS.linux? && Hardware::CPU.arm?
    binary_name = "tap-linux-amd64" if OS.linux? && !Hardware::CPU.arm?

    bin.install binary_name => "tap"
  end

  test do
    system "#{bin}/tap", "--version"
  end
end
