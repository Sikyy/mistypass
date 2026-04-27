import { useState } from "react"
import { ChevronDownIcon, CloudIcon, SearchIcon, Trash2Icon, UserIcon } from "lucide-react"

import { PageFrame, SettingsPanel, StatusDot, ToggleSwitch } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { formatKisiRoleLabel } from "@/features/kisi-shell/navigation"
import type { CurrentUser } from "@/lib/api"

function formatPersonName(email: string) {
  const localPart = email.split("@")[0] || "operator"
  return localPart
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

export function MyAccountPage({ viewer, onLogout }: { viewer: CurrentUser; onLogout: () => void }) {
  const [activeTab, setActiveTab] = useState("Profile")
  const apiKeys = [
    ["Mistyislet Admin API", "Created: 3 days"],
    ["Automation token", "Last used about 3 hours ago"],
    ["Webhook provisioning", "Last used 4 days ago"],
    ["Reader diagnostics", "Last used 12 days ago"],
  ]

  return (
    <PageFrame
      breadcrumbs={["Home", "My Account"]}
      title="My Account"
      description="Manage profile, login, credentials, security, and API keys"
    >
      <SettingsPanel
        tabs={["Profile", "Logins", "Credentials", "Security", "API"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <>
            <Button variant="interaction" className="mr-auto h-10 rounded-[6px] text-[#4f55ff]" onClick={onLogout}>Sign Out</Button>
            <Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">Save</Button>
          </>
        }
      >
        {activeTab === "Profile" ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">Profile</h2>
            </div>
            <div className="grid gap-8 p-7 lg:grid-cols-[1fr_260px]">
              <div className="space-y-6">
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
                    <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#9a9ca7]">
                      <span className="min-w-0 truncate">{formatPersonName(viewer.email)}</span>
                    </div>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Email</span>
                    <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#9a9ca7]">
                      <span className="min-w-0 truncate">{viewer.email}</span>
                    </div>
                  </label>
                </div>
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Account role</span>
                    <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#2f3037]">
                      <span className="min-w-0 truncate">{formatKisiRoleLabel(viewer)}</span>
                    </div>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Language</span>
                    <div className="flex h-12 items-center justify-between rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#2f3037]">
                      English
                      <ChevronDownIcon className="size-4 text-[#6f717c]" />
                    </div>
                  </label>
                </div>
              </div>
              <div className="flex items-center gap-7 lg:justify-center">
                <div className="flex size-28 items-center justify-center rounded-[10px] bg-[#eef0f4]">
                  <UserIcon className="size-14 text-[#17171c]" />
                </div>
                <span className="text-sm font-semibold text-[#6f717c]">{formatKisiRoleLabel(viewer)}</span>
              </div>
            </div>
          </>
        ) : null}

        {activeTab === "Logins" ? (
          <div className="divide-y divide-[#eceef2]">
            <div className="px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">Logins</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Review active sign-in methods and sessions.</p>
            </div>
            {[
              ["Password", "Enabled", "Last changed 18 days ago"],
              ["Single Sign-On", "Organization managed", "SAML is configured by an administrator"],
              ["Active browser session", "Current", "Chrome on macOS"],
            ].map((row, index) => (
              <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_170px_1fr] md:items-center">
                <span className="font-semibold text-[#17171c]">{row[0]}</span>
                <StatusDot tone={index === 1 ? "info" : "success"} label={row[1]} />
                <span className="text-sm text-[#6f717c]">{row[2]}</span>
              </div>
            ))}
          </div>
        ) : null}

        {activeTab === "Credentials" ? (
          <div className="divide-y divide-[#eceef2]">
            <div className="px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">Credentials</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Personal credentials associated with this account.</p>
            </div>
            {[
              ["Mobile credential", "Active", "Used for app unlocks"],
              ["Access link", "Pending", "Expires in 7 days"],
            ].map((row, index) => (
              <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_170px_1fr] md:items-center">
                <span className="font-semibold text-[#17171c]">{row[0]}</span>
                <StatusDot tone={index === 0 ? "success" : "warning"} label={row[1]} />
                <span className="text-sm text-[#6f717c]">{row[2]}</span>
              </div>
            ))}
          </div>
        ) : null}

        {activeTab === "Security" ? (
          <div className="divide-y divide-[#eceef2]">
            <div className="px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">Security</h2>
              <p className="mt-1 text-sm text-[#6f717c]">Account-level protection and recovery settings.</p>
            </div>
            {[
              ["Multi-factor authentication", true, "Require an additional verification step at sign-in."],
              ["Trusted device prompt", true, "Remember this browser after successful MFA."],
              ["Recovery codes", false, "Generate backup access codes for emergency sign-in."],
            ].map(([title, enabled, description]) => (
              <div key={title as string} className="flex gap-5 px-7 py-5">
                <div>
                  <h3 className="font-semibold text-[#17171c]">{title}</h3>
                  <p className="mt-1 text-sm text-[#6f717c]">{description}</p>
                </div>
                <div className="ml-auto pt-1">
                  <ToggleSwitch enabled={Boolean(enabled)} />
                </div>
              </div>
            ))}
          </div>
        ) : null}

        {activeTab === "API" ? (
          <>
            <div className="flex items-center justify-between gap-4 border-b border-[#eceef2] px-7 py-5">
              <div>
                <h2 className="text-lg font-semibold text-[#17171c]">API</h2>
                <p className="mt-1 text-sm text-[#6f717c]">Personal API keys for automation and integrations.</p>
              </div>
              <div className="flex h-11 min-w-0 flex-1 items-center gap-3 rounded-[6px] border border-[#d9dbe3] px-4 lg:max-w-[360px]">
                <SearchIcon className="size-4 text-[#6f717c]" />
                <span className="text-sm text-[#9a9ca7]">Search APIs...</span>
              </div>
              <Button variant="outline" className="h-11 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                Add API Key
              </Button>
            </div>
            <div className="divide-y divide-[#eceef2] px-7">
              {apiKeys.map((item) => (
                <div key={item[0]} className="flex items-center gap-4 py-5">
                  <div className="flex size-10 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                    <CloudIcon className="size-5 text-[#2f3037]" />
                  </div>
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-[#17171c]">{item[0]}</p>
                    <p className="mt-1 text-sm text-[#6f717c]">{item[1]}</p>
                  </div>
                  <button type="button" className="ml-auto flex size-9 items-center justify-center rounded-[6px] text-[#6f717c] hover:bg-[#fbfbfc]" aria-label={`Delete ${item[0]}`}>
                    <Trash2Icon className="size-4" />
                  </button>
                </div>
              ))}
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
