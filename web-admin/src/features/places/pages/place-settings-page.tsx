import { useState } from "react"
import {
  AlertCircleIcon,
  BellIcon,
  ChevronDownIcon,
  CreditCardIcon,
  DoorOpenIcon,
  ServerIcon,
  UsersIcon,
} from "lucide-react"

import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
} from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function PlaceSettingsAdaptedPage() {
  const [activeTab, setActiveTab] = useState("General")

  return (
    <PageFrame
      breadcrumbs={["Home", "Places", "Assigned Place", "Place Settings"]}
      title="Place Settings"
      description="Configure this place's identity, timezone, and access behavior"
    >
      <SettingsPanel
        tabs={["General", "Access", "Schedules", "Notifications", "Advanced"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={<Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">Save</Button>}
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Place name, address, and timezone." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <FormField label="Place name" value="Sudirman Hub" />
              <FormField label="Timezone" value="Asia/Jakarta" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Address" value="Jakarta, Indonesia" />
              <FormField label="Default floor" value="1st Floor" />
            </div>
          </>
        ) : null}

        {activeTab === "Access" ? (
          <>
            <PanelHeader title="Access" description="Place-wide access defaults." />
            <SettingToggleRows
              rows={[
                ["Allow mobile unlocks", true, "Users with valid Access Rights can unlock from mobile apps.", CreditCardIcon],
                ["Require reader unlock for sensitive doors", true, "Apply reader-required mode to high-security groups.", DoorOpenIcon],
                ["Suspend new users by default", false, "Require admin approval before newly synced users can unlock.", UsersIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Schedules" ? (
          <>
            <PanelHeader title="Schedules" description="Reusable schedules for door and group restrictions." />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["Business Hours", "Mon-Fri 08:00-18:00", "Default group schedule"],
                ["After Hours", "Mon-Fri 18:00-22:00", "Facilities only"],
                ["Weekend Maintenance", "Sat-Sun 09:00-16:00", "Temporary vendors"],
              ].map((row) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_220px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <span className="text-sm text-[#2f3037]">{row[1]}</span>
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Notifications" ? (
          <>
            <PanelHeader title="Notifications" description="Place admin alert routing." />
            <SettingToggleRows
              rows={[
                ["Door forced open", true, "Notify place admins and security channel immediately.", BellIcon],
                ["Device offline", true, "Notify after five minutes without heartbeat.", ServerIcon],
                ["Access denied spike", true, "Create a review item when denial thresholds are exceeded.", AlertCircleIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Advanced" ? (
          <>
            <PanelHeader title="Advanced" description="Dangerous actions and data export controls." />
            <div className="space-y-4 p-7">
              {[
                ["Export place audit log", "Generate CSV for unlocks, denials, and admin changes."],
                ["Rotate device secrets", "Force all gateways and readers to refresh credentials."],
                ["Archive place", "Disable new access while preserving historical events."],
              ].map((row, index) => (
                <div key={row[0]} className="flex gap-5 rounded-[6px] border border-[#eceef2] p-5">
                  <div>
                    <h3 className="font-semibold text-[#17171c]">{row[0]}</h3>
                    <p className="mt-1 text-sm text-[#6f717c]">{row[1]}</p>
                  </div>
                  <Button variant="outline" className={cn("ml-auto h-10 rounded-[6px] bg-white px-5", index === 2 ? "border-[#f1b7b2] text-[#d93025]" : "border-[#8589ff] text-[#4f55ff]")}>
                    {index === 0 ? "Export" : index === 1 ? "Rotate" : "Archive"}
                  </Button>
                </div>
              ))}
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
