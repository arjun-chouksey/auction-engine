import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { AuthProvider } from "../auth/AuthContext";

function renderApp(path: string) {
  return render(<MemoryRouter initialEntries={[path]}><AuthProvider><App /></AuthProvider></MemoryRouter>);
}

describe("App", () => {
  it("redirects protected routes to the login screen", () => {
    renderApp("/auctions");
    expect(screen.getByRole("heading", { level: 2, name: "Sign in to the auction room" })).toBeInTheDocument();
    expect(screen.getByText("Demo only.")).toBeInTheDocument();
  });

  it("routes unknown paths to the not-found page", () => {
    renderApp("/missing");

    expect(
      screen.getByRole("heading", { level: 1, name: "Page not found" }),
    ).toBeInTheDocument();
  });
});
