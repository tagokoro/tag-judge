import { describe, expect, it } from "vitest";
import { validateProblemForm, validateSubmissionSource, validateTestCaseForm } from "./validation";

describe("validation", () => {
  it("accepts a valid problem payload", () => {
    const result = validateProblemForm({
      title: "A + B",
      statement: "Solve it",
      time_limit_ms: "1000",
      memory_limit_kb: "262144",
      visibility: "public"
    });

    expect(result.ok).toBe(true);
  });

  it("rejects an invalid test case score", () => {
    const result = validateTestCaseForm({ input: "", expected_output: "", score: "0", is_sample: null });

    expect(result.ok).toBe(false);
  });

  it("rejects blank source code", () => {
    const result = validateSubmissionSource("   ");

    expect(result.ok).toBe(false);
  });
});
