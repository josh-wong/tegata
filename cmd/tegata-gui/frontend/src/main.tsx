import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
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

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
