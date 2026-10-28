import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../auth/AuthContext";
import { App } from "./App";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function mockApplication(role: "BUYER" | "SUPPLIER") {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/v1/auth/login"))
        return Response.json({
          access_token: "test-token",
          user: {
            id: "user-id",
            full_name: `Demo ${role}`,
            email: "demo@example.test",
            role,
          },
        });
      if (url.includes("/api/v1/rfqs?"))
        return Response.json({
          items: [],
          page: 1,
          page_size: 20,
          total_items: 0,
          server_time: "2026-10-28T12:00:00Z",
        });
      return new Response(null, { status: 404 });
    }),
  );
}

async function signInAs(email: string) {
  render(
    <MemoryRouter initialEntries={["/login"]}>
      <AuthProvider>
        <App />
      </AuthProvider>
    </MemoryRouter>,
  );
  fireEvent.change(screen.getByLabelText("Email address"), {
    target: { value: email },
  });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  await screen.findByRole("heading", { level: 1, name: "British Auctions" });
}

describe("role-aware UI", () => {
  it("shows RFQ creation to buyers", async () => {
    mockApplication("BUYER");
    await signInAs("buyer@example.test");
    expect(
      screen.getAllByRole("link", { name: "Create RFQ" }).length,
    ).toBeGreaterThan(0);
  });

  it("does not expose RFQ creation to suppliers", async () => {
    mockApplication("SUPPLIER");
    await signInAs("supplier.a@example.test");
    await waitFor(() =>
      expect(
        screen.queryByRole("link", { name: "Create RFQ" }),
      ).not.toBeInTheDocument(),
    );
  });
});
