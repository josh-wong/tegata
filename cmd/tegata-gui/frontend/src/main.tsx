import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider } from 'react-i18next'
import './styles/globals.css'
import App from './App.tsx'

// Apply the saved theme before React mounts to avoid a flash of the
// wrong color scheme on the setup wizard and unlock screens.
;(() => {
  const theme = localStorage.getItem('tegata-theme') ?? 'system'
  const dark =
    theme === 'dark' ||
    (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
})()

// Disable the default browser context menu globally.
// Sidebar items provide their own custom context menu.
document.addEventListener('contextmenu', (e) => e.preventDefault())

// I18nextProvider here comes from our shim (react-i18next-shim.tsx) via the
// Vite alias. It manages language state synchronously — no async init needed.
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <I18nextProvider>
      <App />
    </I18nextProvider>
  </StrictMode>,
)
