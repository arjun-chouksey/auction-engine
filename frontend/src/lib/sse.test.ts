import { describe, expect, it, vi } from "vitest";
import { consumeRFQStream } from "./sse";

describe("RFQ SSE consumption", () => {
  it("recognizes signals split across chunks and resolves when the server closes", async () => {
    const encoder = new TextEncoder();
    const response = new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(encoder.encode("event: rfq_"));
          controller.enqueue(encoder.encode("changed\r\ndata: {}\r\n\r\n"));
          controller.close();
        },
      }),
    );
    const onChange = vi.fn();
    await consumeRFQStream(response, new AbortController().signal, onChange);
    expect(onChange).toHaveBeenCalledOnce();
  });

  it("rejects non-stream responses so the caller can reconnect", async () => {
    await expect(
      consumeRFQStream(
        new Response(null, { status: 503 }),
        new AbortController().signal,
        vi.fn(),
      ),
    ).rejects.toThrow("unavailable");
  });
});
