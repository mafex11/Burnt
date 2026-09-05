"use client";
import { CopyButton } from "@/components/CopyButton";
import { PlatformToggle } from "@/components/PlatformToggle";
import { usePlatform, type Platform } from "@/components/platform";

export const INSTALL_COMMAND: Record<Platform, string> = {
  macos: "brew install mafex11/tap/burnt",
  windows: "irm https://raw.githubusercontent.com/mafex11/Burnt/main/install.ps1 | iex",
};

const UPDATE_NOTE: Record<Platform, string> = {
  macos: "update later: brew upgrade --cask mafex11/tap/burnt",
  windows: "updates itself from GitHub releases — or rerun the one-liner",
};

const REQUIREMENTS: Record<Platform, string> = {
  macos: "macOS 14+ · Apple Silicon",
  windows: "Windows 10 (1809+) or Windows 11 · x64",
};

const RELEASE_URL = "https://github.com/mafex11/Burnt/releases/latest";
const WINDOWS_ZIP_URL =
  "https://github.com/mafex11/Burnt/releases/latest/download/Burnt-windows-x64.zip";

/** Toggle + copyable one-liner for the hero. */
export function HeroInstall() {
  const [platform] = usePlatform();
  return (
    <div>
      <PlatformToggle />
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <CopyButton text={INSTALL_COMMAND[platform]} />
        <a
          href="https://github.com/mafex11/Burnt"
          className="text-sm font-semibold text-ink hover:text-accent"
        >
          View on GitHub →
        </a>
      </div>
    </div>
  );
}

/** Toggle + command + update / manual-download / first-run notes for the install section. */
export function InstallDetails() {
  const [platform] = usePlatform();
  return (
    <div>
      <div className="flex justify-center">
        <PlatformToggle />
      </div>
      <div className="mt-6 flex justify-center">
        <CopyButton text={INSTALL_COMMAND[platform]} />
      </div>
      <p className="mt-4 font-mono text-xs text-muted">{UPDATE_NOTE[platform]}</p>
      {platform === "macos" ? (
        <p className="mx-auto mt-8 max-w-md text-sm text-muted">
          Prefer a direct download? Grab <code className="text-ink">Burnt.zip</code> from the{" "}
          <a
            href={RELEASE_URL}
            className="text-ink underline decoration-line underline-offset-2 hover:text-accent"
          >
            latest release
          </a>{" "}
          and drag <code className="text-ink">Burnt.app</code> into Applications. Burnt is
          ad-hoc signed, so the first time, right-click the app →{" "}
          <span className="text-ink">Open</span>.
        </p>
      ) : (
        <div className="mx-auto mt-8 max-w-md space-y-3 text-sm text-muted">
          <p>
            Prefer a direct download? Grab{" "}
            <a
              href={WINDOWS_ZIP_URL}
              className="text-ink underline decoration-line underline-offset-2 hover:text-accent"
            >
              <code>Burnt-windows-x64.zip</code>
            </a>{" "}
            (~12 MB — <code className="text-ink">burnt.exe</code> plus{" "}
            <code className="text-ink">ccusage.exe</code>, no Node needed) from the{" "}
            <a
              href={RELEASE_URL}
              className="text-ink underline decoration-line underline-offset-2 hover:text-accent"
            >
              latest release
            </a>
            , unzip it and run <code className="text-ink">burnt.exe</code>. The exe is unsigned,
            so SmartScreen may ask: <span className="text-ink">More info → Run anyway</span>. The
            one-liner unblocks the files for you.
          </p>
          <p>
            The installer puts Burnt in{" "}
            <code className="text-ink">{"%LOCALAPPDATA%\\Programs\\Burnt"}</code>, adds a Start
            Menu shortcut and launch-at-login, and fetches the WebView2 runtime if it&apos;s
            missing (it ships with Windows 11 and most Windows 10).
          </p>
        </div>
      )}
      <p className="mt-6 text-xs text-muted">{REQUIREMENTS[platform]}</p>
    </div>
  );
}
