import { X } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime"

const GITHUB_SPONSORS_URL = "https://github.com/sponsors/josh-wong"
const KO_FI_URL = "https://ko-fi.com/josh_haha"
const PAYPAL_URL = "https://www.paypal.me/joshww"

interface SupportBannerProps {
  onDismiss: () => void
}

export function SupportBanner({ onDismiss }: SupportBannerProps) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-3 border-t bg-muted/50 px-4 py-2 text-sm">
      <span className="shrink-0 text-muted-foreground">{t("gui.settings.supportBannerMessage")}</span>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(GITHUB_SPONSORS_URL)}>
          {t("gui.settings.supportGitHubSponsors")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(KO_FI_URL)}>
          {t("gui.settings.supportKoFi")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(PAYPAL_URL)}>
          {t("gui.settings.supportPayPal")}
        </Button>
      </div>
      <button
        className="ml-auto shrink-0 text-muted-foreground hover:text-foreground"
        onClick={onDismiss}
        aria-label={t("gui.common.dismiss")}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}
