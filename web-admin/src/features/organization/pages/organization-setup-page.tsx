import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  AlertCircleIcon,
  BarChart3Icon,
  BellIcon,
  ChevronDownIcon,
  CloudIcon,
  CreditCardIcon,
  KeyRoundIcon,
  ServerIcon,
  ShieldCheckIcon,
  UsersIcon,
} from "lucide-react"

import {
  FormField,
  PageFrame,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
  ToggleSwitch,
} from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { listIntegrations, type CurrentUser, type Integration } from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

type IntegrationRow = [string, string, string, "success" | "warning" | "info"]

function integrationStatus(integration: Integration): IntegrationRow[3] {
  if (!integration.configured || integration.status === "inactive" || integration.status === "pending") {
    return "warning"
  }
  if (integration.status === "available" || integration.status === "reserved") {
    return "info"
  }
  return "success"
}

function integrationStatusLabel(integration: Integration) {
  if (!integration.configured) {
    return "Review"
  }
  if (integration.status === "active") {
    return "Connected"
  }
  return integration.status.charAt(0).toUpperCase() + integration.status.slice(1)
}

function liveIntegrationRows(integrations: Integration[], activeTab: string, isSsoScim: boolean): IntegrationRow[] {
  const visible = integrations.filter((integration) => {
    if (isSsoScim) {
      return integration.type === "identity_provider"
    }
    if (activeTab === "Directory") {
      return integration.type === "identity_provider" || integration.type === "hris"
    }
    if (activeTab === "Webhooks") {
      return integration.type === "webhook"
    }
    if (activeTab === "MQTT") {
      return integration.type === "mqtt"
    }
    if (activeTab === "Device API") {
      return integration.type === "device_api"
    }
    return false
  })

  return visible.map((integration) => [
    integration.name,
    integrationStatusLabel(integration),
    integration.description,
    integrationStatus(integration),
  ])
}

export function OrganizationSetupAdaptedPage({
  title,
  token,
  viewer,
}: {
  title: string
  token: string
  viewer: CurrentUser
}) {
  const normalized = title.toLowerCase()
  const isCreatePlace = normalized.includes("create")
  const isAlertPolicies = normalized.includes("alert")
  const isIntegrations = normalized.includes("integrations")
  const isBilling = normalized.includes("billing")
  const isSettings = normalized.includes("settings")
  const isSsoScim = normalized.includes("sso")
  const tabs = isAlertPolicies
    ? ["Policies", "Notifications", "Escalation"]
    : isIntegrations
      ? ["Directory", "Webhooks", "MQTT", "Device API"]
      : isSsoScim
        ? ["SSO", "SCIM", "JIT", "Certificates"]
        : isBilling
          ? ["Plan", "Invoices", "Usage"]
          : isCreatePlace
            ? ["General", "Floors", "Doors", "Hardware"]
            : ["General", "Communication", "Security", "Advanced"]
  const [activeTab, setActiveTab] = useState(tabs[0])
  const tenantID = isPlatformViewer(viewer) ? undefined : getViewerTenantID(viewer)
  const integrationsQuery = useQuery({
    queryKey: ["reference-integrations", viewer.id, viewer.role, tenantID ?? "platform"],
    queryFn: () => listIntegrations(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    enabled: isIntegrations || isSsoScim,
    staleTime: 30 * 1000,
  })

  const alertRows =
    activeTab === "Notifications"
      ? [
          ["Security channel", "Enabled", "Send high severity events to #security-ops"],
          ["Place admin email", "Enabled", "Email assigned Place Admins for place-scoped alerts"],
          ["Weekly digest", "Review", "Summarize non-urgent access trends every Monday"],
        ]
      : activeTab === "Escalation"
        ? [
            ["Door forced open", "Enabled", "Escalate after 2 minutes without acknowledgement"],
            ["Gateway offline", "Enabled", "Escalate to hardware owner after 10 minutes"],
            ["Repeated access denied", "Review", "Escalate after 3 denied attempts in 10 minutes"],
          ]
        : [
            ["Door forced open", "Enabled", "High severity, notify security team immediately"],
            ["Gateway offline", "Enabled", "Notify place admins after 5 minutes"],
            ["Repeated access denied", "Review", "Create review item after threshold is met"],
          ]
  const fallbackIntegrationRows: IntegrationRow[] =
    activeTab === "Webhooks"
      ? [
          ["Door events webhook", "Connected", "Unlock, denied, held-open, and forced-open events", "success"],
          ["Credential webhook", "Connected", "Credential issued, revoked, and expired events", "success"],
          ["Retry policy", "Enabled", "Backoff and dead-letter queue enabled", "success"],
          ["Signing secret", "Ready", "Rotate from API settings when needed", "success"],
        ]
      : activeTab === "MQTT"
        ? [
            ["Gateway event stream", "Available", "Reader and controller telemetry stream", "info"],
            ["Device command topic", "Ready", "Reserved for controller operations", "success"],
            ["Broker certificate", "Review", "Certificate expires in 34 days", "warning"],
          ]
        : activeTab === "Device API"
          ? [
              ["Personal API keys", "Available", "Manage user-scoped keys in My Account", "info"],
              ["Service tokens", "Reserved", "Backend API scope for automation", "info"],
              ["Rate limits", "Ready", "Apply per-tenant API protection", "success"],
            ]
          : [
              ["SSO", "Connected", "SAML identity provider for admin sign-in", "success"],
              ["SCIM", "Pending", "Directory sync token has been generated but not confirmed", "warning"],
              ["Webhook", "Connected", "Door, credential, and alarm event delivery", "success"],
              ["MQTT", "Available", "Gateway event stream for device integrations", "info"],
            ]
  const liveRows = liveIntegrationRows(integrationsQuery.data ?? [], activeTab, isSsoScim)
  const integrationRows = liveRows.length > 0 ? liveRows : fallbackIntegrationRows
  const billingRows = [
    ["Current plan", "Active", "Enterprise Access · 3 places · 42 doors"],
    ["Next invoice", "Scheduled", "May 1, 2026"],
    ["Payment method", "Ready", "Visa ending in 4242"],
  ]

  return (
    <PageFrame
      breadcrumbs={["Home", "Organization Setup", title]}
      title={title}
      description="Organization-level configuration in a flat Mistyislet workspace"
    >
      <SettingsPanel
        tabs={tabs}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea]">
            {isCreatePlace ? "Create Place" : "Save Changes"}
          </Button>
        }
      >
        {isCreatePlace ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{activeTab}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Create a new physical place inside this organization.</p>
            </div>
            {activeTab === "General" ? (
              <div className="grid gap-6 p-7 md:grid-cols-2">
                <FormField label="Place name" value="Sudirman Hub" />
                <FormField label="Timezone" value="Asia/Jakarta" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
                <FormField label="Address" value="Jakarta, Indonesia" />
                <FormField label="Default admin" value="Place Admin" />
              </div>
            ) : null}
            {activeTab === "Floors" ? (
              <div className="divide-y divide-[#eceef2]">
                {[
                  ["1st Floor", "Lobby and reception", "Default"],
                  ["3rd Floor", "Office and meeting rooms", "Ready"],
                  ["Parking", "Garage and loading bay", "Ready"],
                ].map((row, index) => (
                  <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_1fr_140px] md:items-center">
                    <span className="font-semibold text-[#17171c]">{row[0]}</span>
                    <span className="text-sm text-[#6f717c]">{row[1]}</span>
                    <StatusDot tone={index === 0 ? "info" : "success"} label={row[2]} />
                  </div>
                ))}
              </div>
            ) : null}
            {activeTab === "Doors" ? (
              <div className="divide-y divide-[#eceef2]">
                {[
                  ["Lobby Turnstile", "1st Floor", "Reader required"],
                  ["Garage", "Parking", "Mobile unlock enabled"],
                  ["11th Floor Entry", "3rd Floor", "Business hours"],
                ].map((row) => (
                  <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_160px_1fr] md:items-center">
                    <span className="font-semibold text-[#17171c]">{row[0]}</span>
                    <span className="text-sm text-[#2f3037]">{row[1]}</span>
                    <span className="text-sm text-[#6f717c]">{row[2]}</span>
                  </div>
                ))}
              </div>
            ) : null}
            {activeTab === "Hardware" ? (
              <SettingToggleRows
                rows={[
                  ["Register gateway after creation", true, "Create an onboarding slot for the first gateway.", ServerIcon],
                  ["Enable reader inventory", true, "Track controller and reader serials from day one.", CreditCardIcon],
                  ["Require firmware baseline", false, "Block activation until devices report approved firmware.", CloudIcon],
                ]}
              />
            ) : null}
          </>
        ) : null}

        {isAlertPolicies ? (
          <>
            <div className="flex items-center justify-between gap-4 border-b border-[#eceef2] px-7 py-5">
              <div>
                <h2 className="text-lg font-semibold text-[#17171c]">{activeTab}</h2>
                <p className="mt-1 text-sm text-[#6f717c]">Define which events create alerts and who receives them.</p>
              </div>
              <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:bg-[#fbfbfc]">
                Add Policy
              </Button>
            </div>
            <div className="divide-y divide-[#eceef2]">
              {alertRows.map((row, index) => (
                <div key={row[0]} className="flex gap-5 px-7 py-5">
                  <div className="flex size-10 shrink-0 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                    <AlertCircleIcon className="size-5 text-[#2f3037]" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="font-semibold text-[#17171c]">{row[0]}</h3>
                    <p className="mt-1 text-sm text-[#6f717c]">{row[2]}</p>
                  </div>
                  <div className="ml-auto flex items-center gap-4">
                    <StatusDot tone={index === 2 ? "warning" : "success"} label={row[1]} />
                    <ToggleSwitch enabled={index !== 2} />
                  </div>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {isIntegrations || isSsoScim ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{activeTab}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Connect identity, directory, webhook, and device event systems.</p>
            </div>
            <div className="grid gap-4 p-7 md:grid-cols-2">
              {(isSsoScim ? integrationRows.slice(0, 2) : integrationRows).map((row, index) => (
                <div key={row[0]} className="rounded-[6px] border border-[#eceef2] p-5">
                  <div className="flex items-start gap-4">
                    <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-[6px]", index === 1 ? "bg-[#fff8ed]" : "bg-[#f1f2f5]")}>
                      <ShieldCheckIcon className="size-5 text-[#2f3037]" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-semibold text-[#17171c]">{row[0]}</h3>
                      <p className="mt-1 text-sm leading-6 text-[#6f717c]">{row[2]}</p>
                      <div className="mt-4">
                        <StatusDot tone={row[3]} label={row[1]} />
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {isBilling ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{activeTab}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Billing is reserved, but the page keeps the same operational structure.</p>
            </div>
            <div className="divide-y divide-[#eceef2]">
              {billingRows.map((row, index) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <StatusDot tone={index === 1 ? "info" : "success"} label={row[1]} />
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {isSettings ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{activeTab}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Organization profile, communication policy, and security defaults.</p>
            </div>
            <div className="space-y-6 p-7">
              {activeTab === "Communication" ? (
                <SettingToggleRows
                  rows={[
                    ["Send account invites", true, "Email users when they are invited to access this organization.", UsersIcon],
                    ["Notify admins about access changes", true, "Send admin-visible updates when Access Rights are changed.", BellIcon],
                    ["Weekly operations digest", false, "Send a weekly summary of places, alerts, and access trends.", BarChart3Icon],
                  ]}
                />
              ) : activeTab === "Security" ? (
                <SettingToggleRows
                  rows={[
                    ["Require MFA for admins", true, "Apply stronger sign-in requirements to Organization Admins.", ShieldCheckIcon],
                    ["Allow WebAuthn", false, "Let users enroll hardware security keys.", KeyRoundIcon],
                    ["Block unmanaged API keys", true, "Require API keys to be created from the approved account workflow.", CloudIcon],
                  ]}
                />
              ) : activeTab === "Advanced" ? (
                <div className="space-y-4">
                  {[
                    ["Export organization audit log", "Generate an organization-wide event archive."],
                    ["Rotate webhook secrets", "Rotate all organization webhook signing secrets."],
                    ["Disable organization", "Reserved destructive action for tenant lifecycle operations."],
                  ].map((row, index) => (
                    <div key={row[0]} className="flex gap-5 rounded-[6px] border border-[#eceef2] p-5">
                      <div>
                        <h3 className="font-semibold text-[#17171c]">{row[0]}</h3>
                        <p className="mt-1 text-sm text-[#6f717c]">{row[1]}</p>
                      </div>
                      <Button variant="outline" className={cn("ml-auto h-10 rounded-[6px] bg-white px-5", index === 2 ? "border-[#f1b7b2] text-[#d93025]" : "border-[#8589ff] text-[#4f55ff]")}>
                        {index === 0 ? "Export" : index === 1 ? "Rotate" : "Disable"}
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="grid gap-6 md:grid-cols-2">
                  {[
                    ["Organization name", "Mistyislet"],
                    ["Primary domain", "mistypass.local"],
                    ["Timezone", "Asia/Jakarta"],
                    ["Support email", "support@mistypass.local"],
                  ].map((row) => (
                    <label key={row[0]} className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{row[0]}</span>
                      <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#2f3037]">{row[1]}</div>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
