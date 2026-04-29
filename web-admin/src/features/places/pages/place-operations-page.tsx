import i18next from "i18next"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { BarChart3Icon, BellIcon, ChevronDownIcon, ShieldCheckIcon } from "lucide-react"

import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
} from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"

export function PlaceOperationsAdaptedPage({ title }: { title: string }) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState("Overview")
  const rows = title === "Integrations"
    ? [
        ["Webhook", "Connected", "Door and unlock events"],
        ["MQTT", "Available", "Reader telemetry stream"],
        ["HRIS Sync", "Organization managed", "Configured at Organization Setup"],
      ]
    : title === "Intrusion Detection"
      ? [
          [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), i18next.t("kisi.orgSetup.description")],
          ["Reader tamper", "Enabled", "Create high severity alert"],
          ["After-hours access", "Review", "Route to reports"],
        ]
      : title === "Capacity Management"
        ? [
            ["Occupancy limit", "Ready", "180 people"],
            ["Visitor threshold", "Ready", "25 guests"],
            ["Export cadence", "Weekly", "Monday 09:00"],
          ]
        : [
            ["Elevator Bank A", "Reserved", "8 floors mapped"],
            ["Service Elevator", "Reserved", "Hardware pending"],
            ["Emergency Override", "Available", "Policy not configured"],
          ]

  return (
    <PageFrame
      breadcrumbs={[t("common.home"), "Places", "Assigned Place", title]}
      title={title}
      description={t("kisi.orgSetup.description")}
    >
      <SettingsPanel
        tabs={["Overview", "Rules", "Events", "Settings"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={<Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">{t("common.save")}</Button>}
      >
        {activeTab === "Overview" ? (
          <>
            <PanelHeader title={t("common.general")} description={`Current ${title.toLowerCase()} configuration for this place.`} />
            <div className="divide-y divide-[#eceef2]">
              {rows.map((row, index) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[240px_160px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <StatusDot tone={index === 2 ? "warning" : "success"} label={row[1]} />
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Rules" ? (
          <>
            <PanelHeader title={t("common.permissions")} description={t("common.permissions")} />
            <SettingToggleRows
              rows={[
                [`Enable ${title}`, title !== "Elevators", `Turn on ${title.toLowerCase()} workflows for this place.`, ShieldCheckIcon],
                [i18next.t("kisi.orgSetup.notifications"), true, i18next.t("kisi.orgSetup.description"), BellIcon],
                ["Include in reports", true, "Expose this area in place analytics and exports.", BarChart3Icon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Events" ? (
          <>
            <PanelHeader title={t("common.events")} description={`Recent ${title.toLowerCase()} events and audit changes.`} />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["12:02 PM", `${title} policy evaluated`, "Success"],
                ["10:44 AM", `${title} configuration updated`, "Success"],
                ["Yesterday", `${title} signal needs review`, "Review"],
              ].map((row, index) => (
                <div key={`${row[0]}-${row[1]}`} className="grid gap-3 px-7 py-5 md:grid-cols-[120px_1fr_140px] md:items-center">
                  <span className="text-sm text-[#6f717c]">{row[0]}</span>
                  <span className="font-semibold text-[#17171c]">{row[1]}</span>
                  <StatusDot tone={index === 2 ? "warning" : "success"} label={row[2]} />
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Settings" ? (
          <>
            <PanelHeader title={t("common.settings")} description={t("common.settings")} />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label={t("kisi.teams.owner")} value="Place Admin" />
              <FormField label={t("common.status")} value="365 days" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label={t("kisi.orgSetup.notifications")} value="#security-sudirman" />
              <FormField label={t("common.scope")} value="Place scoped" />
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
