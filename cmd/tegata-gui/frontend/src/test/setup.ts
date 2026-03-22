import "@testing-library/jest-dom"

vi.mock("@/lib/wails", () => import("./__mocks__/wails"))

// jsdom does not implement window.matchMedia — provide a minimal polyfill
// so hooks like useTheme can call matchMedia without throwing.
Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

// jsdom does not implement navigator.clipboard — stub it for components
// that copy text. Must be configurable so @testing-library/user-event
// can also attach its own clipboard stub.
Object.defineProperty(navigator, "clipboard", {
  writable: true,
  configurable: true,
  value: {
    writeText: vi.fn(() => Promise.resolve()),
    readText: vi.fn(() => Promise.resolve("")),
  },
})
