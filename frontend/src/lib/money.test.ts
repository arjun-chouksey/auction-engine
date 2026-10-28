import { describe, expect, it } from "vitest";
import { decimalToMinor, formatMinor, sumMinor } from "./money";

describe("exact money helpers", () => {
  it("converts decimal input without floating point arithmetic", () => {
    expect(decimalToMinor("1250.50")).toBe("125050");
    expect(decimalToMinor("0.1")).toBe("10");
    expect(sumMinor("9007199254740993", "7")).toBe("9007199254741000");
  });
  it("rejects malformed, negative, and over-precision values", () => {
    expect(() => decimalToMinor("12.345")).toThrow("2 decimal places");
    expect(() => decimalToMinor("-1.00")).toThrow("non-negative");
    expect(() => decimalToMinor("")).toThrow("non-negative");
  });
  it("formats minor-unit strings exactly", () => {
    expect(formatMinor("950000", "INR")).toBe("INR 9,500.00");
    expect(formatMinor(null, "INR")).toBe("—");
  });
});
