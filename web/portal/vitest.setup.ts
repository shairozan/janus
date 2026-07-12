// Adds jest-dom matchers (e.g. toBeInTheDocument) to Vitest's expect, and cleans
// up the rendered DOM between tests.
import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

afterEach(() => {
  cleanup();
});
