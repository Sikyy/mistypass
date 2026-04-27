import { useState } from "react"
import { CameraIcon, ChevronDownIcon, UsersIcon } from "lucide-react"

import {
  FormField,
  InfoBanner,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  StatusDot,
  ToggleSwitch,
} from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"

export function UserDetailAdaptedPage() {
  const [activeTab, setActiveTab] = useState("General")
  const accessRows = [
    ["Service Personnel", "Group", "Garage, 11th Floor Entry", "Weekdays 08:00-18:00"],
    ["Sudirman Hub Admin", "Place role", "All doors", "Always"],
  ]
  const eventRows = [
    ["Apr 26, 12:02 PM", "Garage", "Unlock successful", "Mobile credential"],
    ["Apr 25, 05:44 PM", "Lobby Turnstile", "Access denied", "Outside schedule"],
    ["Apr 24, 09:11 AM", "11th Floor Entry", "Credential issued", "Organization Admin"],
  ]

  return (
    <PageFrame
      breadcrumbs={["Home", "Users"]}
      title="Andra Saputra"
      description="User profile, permissions, credentials, and event history"
      actions={
        <>
          <Button variant="interaction" className="h-10 rounded-[6px] text-[#4f55ff]">
            Share Access
          </Button>
          <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:bg-[#fbfbfc]">
            Delete User
          </Button>
        </>
      }
    >
      <InfoBanner>
        User membership, credentials, and access history stay together on this page. Groups control door restrictions; Teams control bulk access-right assignment.
      </InfoBanner>

      <SettingsPanel
        tabs={["General", "Access Rights", "Credentials", "Logins", "Analytics", "Events"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          activeTab === "General" ? (
            <Button disabled className="h-10 rounded-[8px] bg-[#eef0f4] px-8 text-[#8d909b]">
              Save
            </Button>
          ) : (
            <div className="grid w-full grid-cols-3 text-sm text-[#6f717c]">
              <span>Previous Page</span>
              <span className="text-center text-[#17171c]">Page 1 of 1</span>
              <span className="text-right">Next Page</span>
            </div>
          )
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Basic identity, role, and account status." />
            <div className="grid gap-8 p-7 lg:grid-cols-[1fr_260px]">
              <div className="space-y-6">
                <div className="grid gap-6 md:grid-cols-2">
                  <FormField label="Name" value="Andra Saputra" />
                  <FormField label="Email" value="andra.saputra@tenant.local" muted />
                </div>
                <div className="grid gap-6 md:grid-cols-2">
                  <FormField label="Role" value="Member" />
                  <FormField label="Primary team" value="Engineering" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
                </div>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Notes</span>
                  <div className="flex min-h-[120px] items-start rounded-[6px] border border-[#d9dbe3] px-4 py-3 text-sm text-[#9a9ca7]">
                    Notes
                  </div>
                </label>
                <div className="flex gap-5 rounded-[6px] border border-[#eceef2] p-5">
                  <div className="flex size-10 shrink-0 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                    <UsersIcon className="size-5 text-[#2f3037]" />
                  </div>
                  <div className="max-w-3xl">
                    <p className="font-semibold text-[#17171c]">Suspend Access</p>
                    <p className="mt-1 text-sm leading-6 text-[#6f717c]">
                      Prevent this user from unlocking doors while keeping their account and audit history intact.
                    </p>
                  </div>
                  <div className="ml-auto pt-2">
                    <ToggleSwitch enabled={false} />
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-7 lg:justify-center">
                <div className="flex size-28 items-center justify-center rounded-[10px] bg-[#eef0f4]">
                  <CameraIcon className="size-12 text-[#17171c]" />
                </div>
                <button type="button" className="text-base font-semibold text-[#4f55ff]">
                  Add Photo
                </button>
              </div>
            </div>
          </>
        ) : null}

        {activeTab === "Access Rights" ? (
          <>
            <PanelHeader
              title="Access Rights"
              description="Access Rights connect users, teams, places, groups, and schedules."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Share Access
                </Button>
              }
            />
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left text-sm">
                <thead>
                  <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                    <th className="px-7 py-4 font-semibold">Name</th>
                    <th className="px-4 py-4 font-semibold">Type</th>
                    <th className="px-4 py-4 font-semibold">Scope</th>
                    <th className="px-4 py-4 font-semibold">Schedule</th>
                  </tr>
                </thead>
                <tbody>
                  {accessRows.map((row) => (
                    <tr key={row[0]} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                      <td className="px-7 py-5 font-semibold text-[#4f55ff]">{row[0]}</td>
                      <td className="px-4 py-5 text-[#2f3037]">{row[1]}</td>
                      <td className="px-4 py-5 text-[#6f717c]">{row[2]}</td>
                      <td className="px-4 py-5 text-[#6f717c]">{row[3]}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : null}

        {activeTab === "Credentials" ? (
          <>
            <PanelHeader
              title="Credentials"
              description="Credentials available to this user."
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:bg-[#fbfbfc]">
                  Issue Credential
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["Mobile credential", "Active", "Last used today at Garage"],
                ["Access link", "Pending", "Expires Apr 29, 2026"],
                ["Card credential", "Revoked", "Replaced by mobile credential"],
              ].map((row, index) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <StatusDot tone={index === 0 ? "success" : index === 1 ? "warning" : "danger"} label={row[1]} />
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Logins" ? (
          <>
            <PanelHeader title="Logins" description="Sign-in methods, SSO binding, and recent sessions." />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["Password", "Enabled", "Last changed Apr 08, 2026"],
                ["SSO", "Organization managed", "SAML identity provider"],
                ["Current session", "Active", "Chrome on macOS"],
              ].map((row, index) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_180px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <StatusDot tone={index === 1 ? "info" : "success"} label={row[1]} />
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Analytics" ? (
          <>
            <PanelHeader title="Analytics" description="Per-user access trends and recent denial reasons." />
            <div className="grid gap-4 p-7 md:grid-cols-3">
              {[
                ["Unlocks this week", "38", "6 mobile, 32 reader"],
                ["Denied attempts", "2", "Both outside schedule"],
                ["Most used door", "Garage", "18 unlocks"],
              ].map((item) => (
                <div key={item[0]} className="rounded-[6px] border border-[#eceef2] p-5">
                  <p className="text-sm text-[#6f717c]">{item[0]}</p>
                  <p className="mt-3 text-3xl font-bold text-[#17171c]">{item[1]}</p>
                  <p className="mt-2 text-sm text-[#6f717c]">{item[2]}</p>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Events" ? (
          <>
            <PanelHeader title="Events" description="Recent user-related access and credential events." />
            <div className="divide-y divide-[#eceef2]">
              {eventRows.map((row, index) => (
                <div key={`${row[0]}-${row[1]}`} className="grid gap-3 px-7 py-5 text-sm md:grid-cols-[170px_180px_180px_1fr] md:items-center">
                  <span className="text-[#6f717c]">{row[0]}</span>
                  <span className="font-semibold text-[#17171c]">{row[1]}</span>
                  <StatusDot tone={index === 1 ? "warning" : "success"} label={row[2]} />
                  <span className="text-[#6f717c]">{row[3]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}
      </SettingsPanel>
    </PageFrame>
  )
}
