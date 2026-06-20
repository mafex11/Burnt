import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { CopyButton } from "./CopyButton";

describe("CopyButton", () => {
  beforeEach(() => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  it("copies the exact command and shows Copied", async () => {
    render(<CopyButton text="brew install mafex11/tap/burnt" />);
    await userEvent.click(screen.getByRole("button"));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("brew install mafex11/tap/burnt");
    expect(await screen.findByText("Copied ✓")).toBeTruthy();
  });
});
