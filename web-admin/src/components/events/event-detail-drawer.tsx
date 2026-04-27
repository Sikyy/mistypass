import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { DetailDrawer } from "@/components/detail-drawer"

type EventDetailDrawerEvent = {
  actor: string
  areaID: string
  at: string
  buildingID: string
  detail?: string
  door: string
  gateway: string
  id: string
  raw: unknown
  resultLabel: string
  resultVariant: "default" | "secondary" | "destructive" | "outline"
  tenantLabel: string
  typeLabel: string
}

type EventDetailDrawerProps = {
  event: EventDetailDrawerEvent | null
  onOpenChange: (open: boolean) => void
  open: boolean
}

function formatJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

export function EventDetailDrawer({ event, onOpenChange, open }: EventDetailDrawerProps) {
  const { t, i18n } = useTranslation()

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={event?.id ?? t("events.detail.titleFallback")}
      description={event ? t("events.detail.description") : undefined}
      contentClassName="bg-background text-foreground sm:max-w-xl"
    >
        {event ? (
          <>
            <div className="grid gap-3 rounded-xl border bg-muted/15 p-3 text-sm">
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">{t("events.detail.fields.type")}</span>
                <span className="font-medium">{event.typeLabel}</span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">{t("events.detail.fields.result")}</span>
                <Badge variant={event.resultVariant}>{event.resultLabel}</Badge>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">{t("events.detail.fields.time")}</span>
                <span className="text-right font-medium">{new Date(event.at).toLocaleString(i18n.language)}</span>
              </div>
            </div>

            <div className="grid gap-2 text-sm">
              <div className="rounded-lg border bg-muted/10 p-3">
                <p className="mp-kpi-note">{t("events.detail.fields.tenant")}</p>
                <p className="mt-1 break-all font-medium">{event.tenantLabel}</p>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <div className="rounded-lg border bg-muted/10 p-3">
                  <p className="mp-kpi-note">{t("events.detail.fields.building")}</p>
                  <p className="mt-1 break-all font-medium">{event.buildingID}</p>
                </div>
                <div className="rounded-lg border bg-muted/10 p-3">
                  <p className="mp-kpi-note">{t("events.detail.fields.area")}</p>
                  <p className="mt-1 break-all font-medium">{event.areaID}</p>
                </div>
                <div className="rounded-lg border bg-muted/10 p-3">
                  <p className="mp-kpi-note">{t("events.detail.fields.door")}</p>
                  <p className="mt-1 break-all font-medium">{event.door}</p>
                </div>
                <div className="rounded-lg border bg-muted/10 p-3">
                  <p className="mp-kpi-note">{t("events.detail.fields.gateway")}</p>
                  <p className="mt-1 break-all font-medium">{event.gateway}</p>
                </div>
              </div>
              <div className="rounded-lg border bg-muted/10 p-3">
                <p className="mp-kpi-note">{t("events.detail.fields.actor")}</p>
                <p className="mt-1 break-all font-medium">{event.actor}</p>
              </div>
              {event.detail ? (
                <div className="rounded-lg border bg-muted/10 p-3">
                  <p className="mp-kpi-note">{t("events.detail.fields.detail")}</p>
                  <p className="mt-1 break-words font-medium">{event.detail}</p>
                </div>
              ) : null}
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium">{t("events.detail.rawJSON")}</p>
              <pre className="max-h-[22rem] overflow-auto rounded-xl border bg-black/90 p-3 text-xs leading-5 text-white">
                {formatJSON(event.raw)}
              </pre>
            </div>
          </>
        ) : null}
    </DetailDrawer>
  )
}

export type { EventDetailDrawerEvent }
