import { describe, it, expect, beforeEach } from "vitest";
import { getToken, setToken, clearToken } from "./token-store";

describe("token-store", () => {
  beforeEach(() => localStorage.clear());
  it("round trips a token", () => {
    expect(getToken()).toBeNull();
    setToken("abc");
    expect(getToken()).toBe("abc");
    clearToken();
    expect(getToken()).toBeNull();
  });
});
