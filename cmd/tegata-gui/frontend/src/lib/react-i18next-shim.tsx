// Drop-in replacement for react-i18next that bypasses i18next entirely.
// Uses a simple React context with synchronous nested-key lookup so there
// are no async init issues in any environment (Wails WebView, tests, etc.).

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react"

import enUS from "../locales/en-us.json"
import jaJP from "../locales/ja-jp.json"

export const LANG_STORAGE_KEY = "tegata-language"
export const SUPPORTED_LANGUAGES = ["en-us", "ja-jp"] as const
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number]

type TranslationTree = { [key: string]: string | TranslationTree }

const BUNDLES: Record<string, TranslationTree> = {
  "en-us": enUS as unknown as TranslationTree,
  "ja-jp": jaJP as unknown as TranslationTree,
}

function getNestedValue(obj: TranslationTree, key: string): string | undefined {
  const parts = key.split(".")
  let cur: TranslationTree | string = obj
  for (const part of parts) {
    if (typeof cur !== "object" || cur === null) return undefined
    cur = (cur as TranslationTree)[part]
  }
  return typeof cur === "string" ? cur : undefined
}

function buildT(lang: string) {
  const bundle = BUNDLES[lang] ?? BUNDLES["en-us"]
  return function t(key: string, opts?: Record<string, unknown>): string {
    let resolvedKey = key
    // Handle plural forms: look for _one / _other suffixes
    if (opts && typeof opts.count === "number") {
      const pluralKey = opts.count === 1 ? `${key}_one` : `${key}_other`
      if (getNestedValue(bundle, pluralKey) !== undefined) resolvedKey = pluralKey
    }
    let val = getNestedValue(bundle, resolvedKey)
    if (val === undefined) return key
    if (opts) {
      Object.entries(opts).forEach(([k, v]) => {
        val = (val as string).replace(new RegExp(`{{${k}}}`, "g"), String(v))
      })
    }
    return val as string
  }
}

interface I18nContext {
  language: string
  changeLanguage: (lang: string) => void
}

const Ctx = createContext<I18nContext>({
  language: "en-us",
  changeLanguage: () => {},
})

// Re-export as I18nextProvider so main.tsx import works unchanged.
export function I18nextProvider({
  children,
  i18n: _i18n,
}: {
  children: ReactNode
  i18n?: unknown
}) {
  const [language, setLanguage] = useState<string>(
    typeof localStorage !== "undefined"
      ? (localStorage.getItem(LANG_STORAGE_KEY) ?? "en-us")
      : "en-us",
  )

  function changeLanguage(lang: string) {
    localStorage.setItem(LANG_STORAGE_KEY, lang)
    setLanguage(lang)
  }

  return <Ctx.Provider value={{ language, changeLanguage }}>{children}</Ctx.Provider>
}

export function useTranslation() {
  const { language, changeLanguage } = useContext(Ctx)
  // Memoize so t() is a stable reference within the same language,
  // preventing spurious re-renders in components with t() as a useCallback dep.
  const tFn = useMemo(() => buildT(language), [language])
  const changeLangFn = useCallback(
    (lang: string) => {
      changeLanguage(lang)
      return Promise.resolve()
    },
    [changeLanguage],
  )
  return {
    t: tFn,
    i18n: { language, changeLanguage: changeLangFn },
  }
}

// Stub — components that import initReactI18next don't need it with this shim.
export const initReactI18next = { type: "3rdParty" as const, init: () => {} }

// Standalone t() for class components (e.g. ErrorBoundary) that can't use hooks.
// Always uses the default language since there's no context available.
export function t(key: string, opts?: Record<string, unknown>): string {
  const lang =
    typeof localStorage !== "undefined"
      ? (localStorage.getItem(LANG_STORAGE_KEY) ?? "en-us")
      : "en-us"
  return buildT(lang)(key, opts)
}
