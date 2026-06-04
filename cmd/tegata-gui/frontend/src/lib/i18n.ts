// Translation is handled by react-i18next-shim.tsx (aliased via vite.config.ts).
// This file re-exports the constants that App.tsx and SettingsPanel.tsx import.
export { LANG_STORAGE_KEY, SUPPORTED_LANGUAGES } from "./react-i18next-shim"
export type { SupportedLanguage } from "./react-i18next-shim"

// Stub default export so `import i18n from "@/lib/i18n"` still compiles.
// ErrorBoundary (a class component) uses i18n.t() directly.
import { t } from "./react-i18next-shim"

export { t }

const i18n = {
  t,
  changeLanguage: (_lang: string) => Promise.resolve(),
  language: "en-us",
  isInitialized: true,
}
export default i18n
