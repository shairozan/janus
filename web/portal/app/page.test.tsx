import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import Home from "./page";

// Smoke test proving the component-test toolchain (Vitest + React Testing
// Library + jsdom) works end to end. Real screen tests land in B3.
describe("Home", () => {
  it("renders the portal heading", () => {
    render(<Home />);
    expect(screen.getByRole("heading", { name: /janus management portal/i })).toBeInTheDocument();
  });
});
