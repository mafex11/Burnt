import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { HeroInstall, InstallDetails, INSTALL_COMMAND } from "./PlatformInstall";
import { detectPlatform, resetPlatformStore } from "./platform";

function setUserAgent(ua: string, platformHint?: string) {
  Object.defineProperty(navigator, "userAgent", { value: ua, configurable: true });
  Object.defineProperty(navigator, "userAgentData", {
    value: platformHint === undefined ? undefined : { platform: platformHint },
    configurable: true,
  });
}

describe("platform toggle", () => {
  beforeEach(() => {
    resetPlatformStore();
    window.location.hash = "";
    setUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "macOS");
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  it("defaults to macOS and shows the brew command", () => {
    render(<InstallDetails />);
    expect(screen.getByRole("tab", { name: "macOS" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tab", { name: "Windows" }).getAttribute("aria-selected")).toBe("false");
    expect(screen.getByText(INSTALL_COMMAND.macos)).toBeTruthy();
    expect(screen.queryByText(INSTALL_COMMAND.windows)).toBeNull();
    expect(screen.getByText(/brew upgrade --cask/)).toBeTruthy();
    expect(screen.getByText(/macOS 14\+/)).toBeTruthy();
  });

  it("switching to Windows updates the command and the notes", async () => {
    render(<InstallDetails />);
    await userEvent.click(screen.getByRole("tab", { name: "Windows" }));

    expect(screen.getByRole("tab", { name: "Windows" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByText(INSTALL_COMMAND.windows)).toBeTruthy();
    expect(screen.queryByText(INSTALL_COMMAND.macos)).toBeNull();
    expect(screen.getByText(/updates itself from GitHub releases/)).toBeTruthy();
    expect(screen.getByText(/Windows 10 \(1809\+\) or Windows 11/)).toBeTruthy();
    expect(screen.getByText(/SmartScreen/)).toBeTruthy();
    expect(screen.getByText(/WebView2/)).toBeTruthy();
    expect(
      screen.getByRole("link", { name: /Burnt-windows-x64\.zip/ }).getAttribute("href"),
    ).toBe(
      "https://github.com/mafex11/Burnt/releases/latest/download/Burnt-windows-x64.zip",
    );
  });

  it("copies the selected platform's command", async () => {
    render(<InstallDetails />);
    await userEvent.click(screen.getByRole("tab", { name: "Windows" }));
    await userEvent.click(screen.getByRole("button", { name: "Copy install command" }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(INSTALL_COMMAND.windows);
  });

  it("keeps hero and install sections in sync", async () => {
    render(
      <>
        <HeroInstall />
        <InstallDetails />
      </>,
    );
    expect(screen.getAllByText(INSTALL_COMMAND.macos)).toHaveLength(2);

    await userEvent.click(screen.getAllByRole("tab", { name: "Windows" })[0]);

    expect(screen.getAllByText(INSTALL_COMMAND.windows)).toHaveLength(2);
    expect(
      screen.getAllByRole("tab", { name: "Windows" }).map((t) => t.getAttribute("aria-selected")),
    ).toEqual(["true", "true"]);
  });

  it("detects Windows from the browser on mount", async () => {
    setUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "Windows");
    render(<InstallDetails />);
    expect(await screen.findByText(INSTALL_COMMAND.windows)).toBeTruthy();
  });

  it("detects the platform from userAgent when userAgentData is missing", () => {
    setUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)", undefined);
    expect(detectPlatform()).toBe("windows");
    setUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", undefined);
    expect(detectPlatform()).toBe("macos");
  });

  it("honours a #windows URL hash", () => {
    window.location.hash = "#windows";
    expect(detectPlatform()).toBe("windows");
  });
});
