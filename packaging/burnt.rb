cask "burnt" do
  version "1.0.0"
  sha256 :no_check # replace with real sha256 once a release zip is published

  url "https://github.com/mafex11/Burnt/releases/download/v#{version}/Burnt.zip"
  name "Burnt"
  desc "Menu bar tracker for Claude Code and Codex token usage and cost"
  homepage "https://github.com/mafex11/Burnt"

  # No node dependency — ccusage is bundled inside the app.
  depends_on macos: ">= :sonoma"

  app "Burnt.app"

  caveats <<~EOS
    Burnt is ad-hoc signed (not notarized). On first launch, macOS may say it
    "cannot be opened". Right-click Burnt.app → Open, then click Open again —
    a one-time step. Or: System Settings → Privacy & Security → Open Anyway.
  EOS

  zap trash: [
    "~/Library/Preferences/dev.mafex.burnt.plist",
  ]
end
