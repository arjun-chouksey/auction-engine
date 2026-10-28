import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("API error handling", () => {
  it("preserves the server domain code and safe message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "AUCTION_CLOSED", message: "Auction is no longer accepting bids" } }), { status: 409, headers: { "Content-Type": "application/json" } })));
    await expect(api("/rfqs/example")).rejects.toEqual(expect.objectContaining({ status: 409, code: "AUCTION_CLOSED", message: "Auction is no longer accepting bids" }));
  });

  it("turns a network failure into a retryable public error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("socket detail")));
    await expect(api("/rfqs")).rejects.toEqual(expect.objectContaining({ status: 0, code: "NETWORK_ERROR", message: "The server could not be reached. Check your connection and try again." }));
  });
});
