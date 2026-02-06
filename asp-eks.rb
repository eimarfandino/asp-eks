class AspEks < Formula
  desc "AWS profile switcher for EKS CLI access"
  homepage "https://github.com/eimarfandino/asp-eks"
  version "0.9.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/eimarfandino/asp-eks/releases/download/v#{version}/asp-eks_v#{version}_darwin_amd64.zip"
      sha256 "REPLACE_WITH_ARM64_SHA256"
    else
      url "https://github.com/eimarfandino/asp-eks/releases/download/v#{version}/asp-eks_v#{version}_darwin_amd64.zip"
      sha256 "REPLACE_WITH_AMD64_SHA256"
    end
  end

  def install
    system "unzip", "-q", Dir["asp-eks_v*_darwin_*.zip"].first if Dir["*.zip"].any?
    bin.install "asp-eks"
  end

  test do
    system "#{bin}/asp-eks", "--help"
  end
end