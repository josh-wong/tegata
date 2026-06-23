import { X } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { SUPPORT_URLS } from "@/lib/constants"
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime"

interface SupportBannerProps {
  onDismiss: () => void
  onDismissPermanently: () => void
}

export function SupportBanner({ onDismiss, onDismissPermanently }: SupportBannerProps) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-3 border-t bg-muted/50 px-4 py-2 text-sm">
      <span className="shrink-0 text-muted-foreground">{t("gui.settings.supportBannerMessage")}</span>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(SUPPORT_URLS.gitHubSponsors)}>
          {t("gui.settings.supportGitHubSponsors")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(SUPPORT_URLS.koFi)}>
          {t("gui.settings.supportKoFi")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => BrowserOpenURL(SUPPORT_URLS.payPal)}>
          {t("gui.settings.supportPayPal")}
        </Button>
        <Button variant="ghost" size="sm" onClick={onDismissPermanently}>
          {t("gui.settings.supportBannerDontShowAgain")}
        </Button>
      </div>
      <Button
        variant="ghost"
        size="sm"
        className="ml-auto shrink-0"
        onClick={onDismiss}
        aria-label={t("gui.common.dismiss")}
      >
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}
