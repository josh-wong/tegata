import { describe, expect, it } from "vitest"
import { formatError } from "./utils"

describe("formatError", () => {
  it("returns the string when given a string", () => {
    expect(formatError("some error text", "fallback")).toBe("some error text")
  })

  it("returns the message when given an Error", () => {
    expect(formatError(new Error("error msg"), "fallback")).toBe("error msg")
  })

  it("returns the fallback when given a number", () => {
    expect(formatError(42, "fallback")).toBe("fallback")
  })

  it("returns the fallback when given null", () => {
    expect(formatError(null, "fallback")).toBe("fallback")
  })

  it("returns the fallback when given undefined", () => {
    expect(formatError(undefined, "fallback")).toBe("fallback")
  })

  it("returns the fallback when given an object", () => {
    expect(formatError({ code: 500 }, "fallback")).toBe("fallback")
  })

  it("returns an empty string when given an empty string", () => {
    expect(formatError("", "fallback")).toBe("")
  })

  it("returns empty message from Error with empty string, not fallback", () => {
    expect(formatError(new Error(""), "fallback")).toBe("")
  })
})
