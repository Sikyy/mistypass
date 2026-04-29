import { useEffect, useMemo, useState } from "react"
import i18next from "i18next"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronDownIcon, CloudIcon, SearchIcon, ShieldCheckIcon, Trash2Icon, UserIcon } from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, SettingsPanel, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { formatMistyisletRoleLabel } from "@/features/mistyislet-shell/navigation"
import {
  disableUserMFA,
  enableUserMFA,
  getUserMFAStatus,
  setupUserMFA,
  updateCurrentUser,
  type CurrentUser,
} from "@/lib/api"

function formatPersonName(email: string) {
  const localPart = email.split("@")[0] || "operator"
  return localPart
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

const languageOptions = [
  { value: "en-US", label: "English" },
  { value: "id-ID", label: "Bahasa Indonesia" },
  { value: "zh-CN", label: "Chinese (Simplified)" },
]

function normalizedProfileLanguage(language: string | undefined): string {
  const nextLanguage = language?.trim()
  switch (nextLanguage) {
    case "en-US":
    case "id-ID":
    case "zh-CN":
      return nextLanguage
    default:
      return "en-US"
  }
}

function profileNameForViewer(viewer: CurrentUser) {
  return viewer.name?.trim() || formatPersonName(viewer.email)
}

type MyAccountPageProps = {
  token: string
  viewer: CurrentUser
  onViewerChange: (viewer: CurrentUser) => void
  onLogout: () => void
}

export function MyAccountPage({ token, viewer, onViewerChange, onLogout }: MyAccountPageProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState(t("kisi.myAccount.profile"))
  const [profileName, setProfileName] = useState(() => profileNameForViewer(viewer))
  const [profileLanguage, setProfileLanguage] = useState(() => normalizedProfileLanguage(viewer.language))
  const [profileFeedback, setProfileFeedback] = useState<{ tone: "success" | "error"; message: string } | null>(null)
  const apiKeys = [
    ["Mistyislet Admin API", "Created: 3 days"],
    ["Automation token", "Last used about 3 hours ago"],
    ["Webhook provisioning", "Last used 4 days ago"],
    ["Reader diagnostics", "Last used 12 days ago"],
  ]
  const savedProfile = useMemo(
    () => ({
      name: profileNameForViewer(viewer),
      language: normalizedProfileLanguage(viewer.language),
    }),
    [viewer]
  )
  const profileDirty = profileName.trim() !== savedProfile.name || profileLanguage !== savedProfile.language
  const profileMutation = useMutation({
    mutationFn: () =>
      updateCurrentUser(token, {
        name: profileName.trim(),
        language: profileLanguage,
      }),
    onSuccess: (updatedViewer) => {
      onViewerChange(updatedViewer)
      setProfileName(profileNameForViewer(updatedViewer))
      setProfileLanguage(normalizedProfileLanguage(updatedViewer.language))
      setProfileFeedback({ tone: "success", message: i18next.t("kisi.myAccount.profileSaved") })
    },
    onError: (error) => {
      setProfileFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : "Unable to save profile",
      })
    },
  })

  useEffect(() => {
    setProfileName(profileNameForViewer(viewer))
    setProfileLanguage(normalizedProfileLanguage(viewer.language))
  }, [viewer])

  function handleProfileSave() {
    if (!profileDirty || !profileName.trim() || profileMutation.isPending) {
      return
    }
    profileMutation.mutate()
  }

  return (
    <PageFrame
      breadcrumbs={[t("common.home"), "My Account"]}
      title={t("kisi.myAccount.title")}
      description={t("kisi.myAccount.description")}
    >
      <SettingsPanel
        tabs={[t("kisi.myAccount.profile"), t("kisi.myAccount.logins"), "Credentials", t("kisi.myAccount.security"), "API"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <>
            <Button variant="interaction" className="mr-auto h-10 rounded-[6px] text-[#4f55ff]" onClick={onLogout}>{t("kisi.shell.signOut")}</Button>
            <Button
              disabled={activeTab !== t("kisi.myAccount.profile") || !profileDirty || !profileName.trim() || profileMutation.isPending}
              className="h-10 rounded-[8px] px-8"
              onClick={handleProfileSave}
            >
              {profileMutation.isPending ? "Saving..." : "Save"}
            </Button>
          </>
        }
      >
        {activeTab === t("kisi.myAccount.profile") ? (
          <>
            <div className="border-b border-[#eceef2] px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.myAccount.profile")}</h2>
            </div>
            <div className="grid gap-8 p-7 lg:grid-cols-[1fr_260px]">
              <div className="space-y-6">
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
                    <input
                      value={profileName}
                      onChange={(event) => {
                        setProfileName(event.target.value)
                        setProfileFeedback(null)
                      }}
                      className="h-12 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm text-[#2f3037] outline-none transition focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.email")}</span>
                    <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#9a9ca7]">
                      <span className="min-w-0 truncate">{viewer.email}</span>
                    </div>
                  </label>
                </div>
                <div className="grid gap-6 md:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.myAccount.role")}</span>
                    <div className="flex h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm text-[#2f3037]">
                      <span className="min-w-0 truncate">{formatMistyisletRoleLabel(viewer)}</span>
                    </div>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.myAccount.language")}</span>
                    <div className="relative">
                      <select
                        value={profileLanguage}
                        onChange={(event) => {
                          setProfileLanguage(event.target.value)
                          setProfileFeedback(null)
                        }}
                        className="h-12 w-full appearance-none rounded-[6px] border border-[#d9dbe3] bg-white px-4 pr-10 text-sm text-[#2f3037] outline-none transition focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
                      >
                        {languageOptions.map((language) => (
                          <option key={language.value} value={language.value}>
                            {language.label}
                          </option>
                        ))}
                      </select>
                      <ChevronDownIcon className="pointer-events-none absolute top-1/2 right-4 size-4 -translate-y-1/2 text-[#6f717c]" />
                    </div>
                  </label>
                </div>
                {profileFeedback ? (
                  <p
                    role={profileFeedback.tone === "error" ? "alert" : "status"}
                    className={
                      profileFeedback.tone === "error"
                        ? "text-sm font-medium text-[#b42318]"
                        : "text-sm font-medium text-[#057a55]"
                    }
                  >
                    {profileFeedback.message}
                  </p>
                ) : null}
              </div>
              <div className="flex items-center gap-7 lg:justify-center">
                <div className="flex size-28 items-center justify-center rounded-[10px] bg-[#eef0f4]">
                  <UserIcon className="size-14 text-[#17171c]" />
                </div>
                <span className="text-sm font-semibold text-[#6f717c]">{formatMistyisletRoleLabel(viewer)}</span>
              </div>
            </div>
          </>
        ) : null}

        {activeTab === t("kisi.myAccount.logins") ? (
          <div className="divide-y divide-[#eceef2]">
            <div className="px-7 py-5">
              <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.myAccount.logins")}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.myAccount.description")}</p>
            </div>
            {[
              [i18next.t("common.status"), i18next.t("common.enabled"), "Last changed 18 days ago"],
              ["SSO", i18next.t("common.organization"), "SAML is configured by an administrator"],
              [i18next.t("common.active"), i18next.t("common.status"), "Chrome on macOS"],
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
              <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.myAccount.credentials")}</h2>
              <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.myAccount.credentials")}</p>
            </div>
            {[
              [i18next.t("kisi.credentials.title"), i18next.t("common.active"), "Used for app unlocks"],
              [i18next.t("kisi.accessRights.accessLink"), i18next.t("common.scheduled"), "Expires in 7 days"],
            ].map((row, index) => (
              <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_170px_1fr] md:items-center">
                <span className="font-semibold text-[#17171c]">{row[0]}</span>
                <StatusDot tone={index === 0 ? "success" : "warning"} label={row[1]} />
                <span className="text-sm text-[#6f717c]">{row[2]}</span>
              </div>
            ))}
          </div>
        ) : null}

        {activeTab === t("kisi.myAccount.security") ? (
          <MFASecurityTab token={token} />
        ) : null}

        {activeTab === "API" ? (
          <>
            <div className="flex items-center justify-between gap-4 border-b border-[#eceef2] px-7 py-5">
              <div>
                <h2 className="text-lg font-semibold text-[#17171c]">API</h2>
                <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.myAccount.api")}</p>
              </div>
              <div className="flex h-11 min-w-0 flex-1 items-center gap-3 rounded-[6px] border border-[#d9dbe3] px-4 lg:max-w-[360px]">
                <SearchIcon className="size-4 text-[#6f717c]" />
                <span className="text-sm text-[#9a9ca7]">{t("common.search")}</span>
              </div>
              <Button variant="outline" className="h-11 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]">
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

function MFASecurityTab({ token }: { token: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [mfaStep, setMfaStep] = useState<"idle" | "setup" | "verify">("idle")
  const [otpauthUrl, setOtpauthUrl] = useState("")
  const [totpSecret, setTotpSecret] = useState("")
  const [verifyCode, setVerifyCode] = useState("")
  const [mfaError, setMfaError] = useState("")
  const [confirmDisable, setConfirmDisable] = useState(false)
  const [mfaNotice, setMfaNotice] = useState("")

  const mfaStatusQuery = useQuery({
    queryKey: ["mfa-status"],
    queryFn: () => getUserMFAStatus(token),
    enabled: !!token,
  })

  const setupMutation = useMutation({
    mutationFn: () => setupUserMFA(token),
    onSuccess: (enrollment) => {
      setOtpauthUrl(enrollment.otpauth_url)
      setTotpSecret(enrollment.secret)
      setMfaStep("verify")
      setMfaError("")
    },
    onError: () => setMfaError("Failed to start MFA setup."),
  })

  const enableMutation = useMutation({
    mutationFn: (code: string) => enableUserMFA(token, code),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mfa-status"] })
      setMfaStep("idle")
      setVerifyCode("")
      setOtpauthUrl("")
      setTotpSecret("")
      setMfaNotice(t("kisi.myAccount.mfaEnabled"))
      setMfaError("")
    },
    onError: () => setMfaError("Invalid code. Please try again."),
  })

  const disableMutation = useMutation({
    mutationFn: () => disableUserMFA(token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mfa-status"] })
      setConfirmDisable(false)
      setMfaNotice(t("kisi.myAccount.mfaDisabled"))
    },
  })

  const mfaEnabled = mfaStatusQuery.data?.enabled ?? false

  return (
    <div className="divide-y divide-[#eceef2]">
      <div className="px-7 py-5">
        <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.myAccount.security")}</h2>
        <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.myAccount.mfaDescription")}</p>
      </div>

      {mfaNotice && (
        <div className="mx-7 mt-2 rounded-[6px] border border-[#b7e4c7] bg-[#f0faf4] px-5 py-3 text-sm text-[#1a7f37]">
          {mfaNotice}
        </div>
      )}

      <div className="px-7 py-5">
        <div className="flex items-start gap-4">
          <div className="flex size-10 items-center justify-center rounded-[6px] bg-[#f3f4ff]">
            <ShieldCheckIcon className="size-5 text-[#4f55ff]" />
          </div>
          <div className="flex-1">
            <h3 className="font-semibold text-[#17171c]">{t("kisi.myAccount.mfaTitle")}</h3>
            <p className="mt-1 text-sm text-[#6f717c]">
              {mfaEnabled ? t("kisi.myAccount.mfaEnabled") : t("kisi.myAccount.mfaDisabled")}
            </p>

            {mfaStep === "idle" && !mfaEnabled && (
              <Button
                className="mt-4 h-10 rounded-[6px] px-6"
                onClick={() => { setMfaNotice(""); setupMutation.mutate() }}
                disabled={setupMutation.isPending}
              >
                {setupMutation.isPending ? "Setting up..." : t("kisi.myAccount.mfaEnable")}
              </Button>
            )}

            {mfaStep === "idle" && mfaEnabled && (
              <Button
                variant="outline"
                className="mt-4 h-10 rounded-[6px] border-[#f1b7b2] px-6 text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff5f5] hover:text-[#9f1d1d]"
                onClick={() => { setMfaNotice(""); setConfirmDisable(true) }}
              >
                {t("kisi.myAccount.mfaDisable")}
              </Button>
            )}

            {mfaStep === "verify" && (
              <div className="mt-4 space-y-4">
                <div className="rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-5">
                  <p className="mb-3 text-sm font-semibold text-[#17171c]">{t("kisi.myAccount.mfaScanQR")}</p>
                  <div className="rounded-[6px] border border-[#d9dbe3] bg-white p-4">
                    <code className="block break-all text-xs text-[#6f717c]">{otpauthUrl}</code>
                  </div>
                  <p className="mt-3 text-xs text-[#6f717c]">
                    {t("kisi.myAccount.mfaManualEntry")} <code className="font-mono text-[#17171c]">{totpSecret}</code>
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.myAccount.mfaEnterCode")}</label>
                  <div className="flex gap-3">
                    <input
                      value={verifyCode}
                      onChange={(e) => { setVerifyCode(e.target.value.replace(/\D/g, "").slice(0, 6)); setMfaError("") }}
                      placeholder="000000"
                      maxLength={6}
                      className="h-12 w-40 rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-center font-mono text-lg tracking-[0.3em] text-[#17171c] outline-none transition focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
                    />
                    <Button
                      className="h-12 rounded-[6px] px-6"
                      disabled={verifyCode.length !== 6 || enableMutation.isPending}
                      onClick={() => enableMutation.mutate(verifyCode)}
                    >
                      {enableMutation.isPending ? "Verifying..." : "Verify"}
                    </Button>
                    <Button
                      variant="outline"
                      className="h-12 rounded-[6px] px-4"
                      onClick={() => { setMfaStep("idle"); setVerifyCode(""); setMfaError("") }}
                    >
                      Cancel
                    </Button>
                  </div>
                  {mfaError && <p className="mt-2 text-sm text-[#d93025]">{mfaError}</p>}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <ConfirmActionDialog
        open={confirmDisable}
        title={t("kisi.myAccount.mfaDisable")}
        description={t("kisi.myAccount.mfaDisableConfirm")}
        confirmLabel={t("kisi.myAccount.mfaDisable")}
        onConfirm={() => disableMutation.mutate()}
        onOpenChange={(open) => { if (!open) setConfirmDisable(false) }}
        pending={disableMutation.isPending}
        destructive
      />
    </div>
  )
}
