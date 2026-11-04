import { describe, it, expect } from "vitest";
import { formatDateTime } from "./format";

describe("formatDateTime", () => {
  it("formats an ISO string into a non-empty locale string", () => {
    const out = formatDateTime("2026-06-29T10:00:00Z");
    expect(out).toBeTruthy();
    expect(out).not.toBe("2026-06-29T10:00:00Z");
  });
  it("returns empty string for falsy input", () => {
    expect(formatDateTime("")).toBe("");
  });
});
