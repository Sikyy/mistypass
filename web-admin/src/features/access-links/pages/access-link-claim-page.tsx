import { type FormEvent, useMemo, useState } from "react"
import i18next from "i18next"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams, useSearchParams } from "react-router"
import {
  ArrowRightIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  KeyRoundIcon,
  LinkIcon,
  ShieldCheckIcon,
  XCircleIcon,
} from "lucide-react"

import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { verifyGroupLinkToken, type GroupLinkVerification } from "@/lib/api"

const claimSummaryItems = [
  { label: "Token", description: "Secret or QR token", Icon: KeyRoundIcon },
  { label: "Window", description: "Valid schedule check", Icon: CalendarClockIcon },
  { label: "Audit", description: "Last used timestamp", Icon: LinkIcon },
]

function firstTokenValue(values: Array<string | undefined | null>) {
  for (const value of values) {
    const trimmed = value?.trim()
    if (trimmed) {
      return trimmed
    }
  }
  return ""
}

function formatDateTime(value?: string) {
  const trimmed = value?.trim()
  if (!trimmed) {
    return i18next.t("common.noDataFound")
  }
  const timestamp = new Date(trimmed)
  if (Number.isNaN(timestamp.getTime())) {
    return trimmed
  }
  return timestamp.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  })
}

function statusMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message
  }
  return i18next.t("kisi.accessLink.unavailable")
}

function AccessLinkDetail({ result }: { result: GroupLinkVerification }) {
  const { t } = useTranslation()
  const link = result.group_link
  const details = [
    ["Access group", link.group_name || link.group_id],
    ["Link name", link.name],
    ["Valid from", formatDateTime(link.valid_from)],
    ["Valid until", formatDateTime(link.valid_until)],
    ["Verified at", formatDateTime(result.verified_at)],
    ["Claimed at", formatDateTime(result.claimed_at || link.last_used_at)],
  ]

  return (
    <div className="space-y-5">
      <div className="rounded-[6px] border border-emerald-400/30 bg-emerald-400/10 px-4 py-3 text-sm text-emerald-100">
        <div className="flex items-start gap-3">
          <CheckCircle2Icon className="mt-0.5 size-5 shrink-0" />
          <div>
            <p className="font-semibold text-emerald-50">{t("kisi.accessLink.verified")}</p>
            <p className="mt-1 text-emerald-100/80">{t("kisi.accessLink.verifiedDesc")}</p>
          </div>
        </div>
      </div>

      <div className="grid gap-3">
        {details.map(([label, value]) => (
          <div key={label} className="flex items-start justify-between gap-4 border-b border-white/10 pb-3 last:border-0 last:pb-0">
            <span className="text-xs font-semibold text-muted-foreground">{label}</span>
            <span className="max-w-[14rem] text-right text-sm font-medium text-foreground sm:max-w-xs">{value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

export function AccessLinkClaimPage() {
  const { t } = useTranslation()
  const params = useParams<{ token?: string }>()
  const [searchParams] = useSearchParams()
  const urlToken = useMemo(
    () =>
      firstTokenValue([
        params.token,
        searchParams.get("token"),
        searchParams.get("secret"),
        searchParams.get("quick_response_code_token"),
      ]),
    [params.token, searchParams]
  )
  const [manualToken, setManualToken] = useState(urlToken)
  const [submittedToken, setSubmittedToken] = useState(urlToken)
  const tokenToVerify = submittedToken.trim()
  const verificationQuery = useQuery({
    queryKey: ["access-link-claim", tokenToVerify],
    queryFn: () => verifyGroupLinkToken(undefined, { token: tokenToVerify }),
    enabled: tokenToVerify.length > 0,
    retry: false,
  })

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmittedToken(manualToken.trim())
  }

  const showEmptyState = tokenToVerify.length === 0
  const isVerified = Boolean(verificationQuery.data?.valid)

  return (
    <div className="mp-fog-surface min-h-screen bg-background px-4 py-6 sm:px-6">
      <main className="mx-auto flex min-h-[calc(100vh-3rem)] w-full max-w-5xl flex-col">
        <header className="flex items-center justify-between gap-4 py-4">
          <div className="flex items-center gap-3">
            <MistyIslandMark className="size-12" markClassName="h-10 w-14" />
            <div>
              <p className="text-[11px] font-semibold uppercase text-muted-foreground">Mistyislet</p>
              <p className="text-sm text-foreground">{t("kisi.accessLink.accessLink")}</p>
            </div>
          </div>
          <Button
            asChild
            variant="outline"
            size="sm"
            className="border-white/10 bg-white/[0.045] text-foreground hover:border-white/20 hover:bg-white/[0.075] hover:text-foreground"
          >
            <Link to="/login">
              <KeyRoundIcon className="mr-1.5 size-4" />
              Sign in
            </Link>
          </Button>
        </header>

        <section className="grid flex-1 items-center gap-8 py-8 lg:grid-cols-[minmax(0,1fr)_25rem]">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2 rounded-[6px] border border-white/10 bg-white/[0.045] px-3 py-2 text-xs font-semibold text-muted-foreground">
              <ShieldCheckIcon className="size-4 text-emerald-200" />
              Secure access verification
            </div>
            <div className="space-y-4">
              <h1 className="max-w-xl text-4xl font-semibold leading-tight text-foreground md:text-5xl">{t("kisi.accessLink.claimTitle")}</h1>
              <p className="max-w-lg text-sm leading-6 text-muted-foreground">
                Verify an access link before it is added to a group invitation record.
              </p>
            </div>
            <div className="grid max-w-xl gap-3 sm:grid-cols-3">
              {claimSummaryItems.map(({ label, description, Icon }) => (
                <div key={label} className="rounded-[6px] border border-white/10 bg-white/[0.045] p-4">
                  <Icon className="mb-3 size-5 text-white/75" />
                  <p className="text-sm font-semibold text-foreground">{label}</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-[6px] border border-white/10 bg-card p-5 shadow-2xl shadow-black/30">
            <form onSubmit={onSubmit} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="access-link-token">{t("kisi.accessLink.accessToken")}</Label>
                <Input
                  id="access-link-token"
                  value={manualToken}
                  onChange={(event) => setManualToken(event.target.value)}
                  placeholder="gls_... or glq_..."
                  autoComplete="off"
                />
              </div>
              <Button type="submit" className="w-full" disabled={!manualToken.trim() || verificationQuery.isFetching}>
                {verificationQuery.isFetching ? t("common.loading") : t("kisi.accessLink.accessToken")}
                <ArrowRightIcon className="ml-1.5 size-4" />
              </Button>
            </form>

            <div className="mt-5 border-t border-white/10 pt-5">
              {showEmptyState ? (
                <div className="rounded-[6px] border border-white/10 bg-white/[0.035] px-4 py-3 text-sm text-muted-foreground">
                  Paste an access token to verify the invitation.
                </div>
              ) : null}

              {verificationQuery.isFetching ? (
                <div className="rounded-[6px] border border-white/10 bg-white/[0.035] px-4 py-3 text-sm text-muted-foreground">
                  Checking this access link...
                </div>
              ) : null}

              {verificationQuery.error && !verificationQuery.isFetching ? (
                <div className="rounded-[6px] border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  <div className="flex items-start gap-3">
                    <XCircleIcon className="mt-0.5 size-5 shrink-0" />
                    <div>
                      <p className="font-semibold">{t("kisi.accessLink.unavailable")}</p>
                      <p className="mt-1 text-destructive/80">{statusMessage(verificationQuery.error)}</p>
                    </div>
                  </div>
                </div>
              ) : null}

              {isVerified && verificationQuery.data ? <AccessLinkDetail result={verificationQuery.data} /> : null}
            </div>
          </div>
        </section>
      </main>
    </div>
  )
}
