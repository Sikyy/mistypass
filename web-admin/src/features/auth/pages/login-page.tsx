import type { TFunction } from "i18next"
import { ArrowRightIcon, Building2Icon, LockKeyholeIcon } from "lucide-react"
import { type FormEvent, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { useAuth } from "@/context/auth-context"
import { login } from "@/lib/api"

const languageOptions = [
  { code: "zh-CN", labelKey: "common.language.zh" },
  { code: "en-US", labelKey: "common.language.en" },
  { code: "id-ID", labelKey: "common.language.id" },
] as const

function buildLoginSubmitSchema(t: TFunction) {
  return z.object({
    email: z
      .string()
      .trim()
      .min(1, t("login.validation.emailRequired"))
      .email(t("login.validation.emailInvalid"))
      .max(320, t("login.validation.emailTooLong")),
    password: z
      .string()
      .min(1, t("login.validation.passwordRequired"))
      .max(128, t("login.validation.passwordTooLong")),
  })
}

type LoginSubmitFormValues = z.infer<ReturnType<typeof buildLoginSubmitSchema>>

function isLanguageActive(current: string, candidate: string) {
  if (current === candidate) {
    return true
  }
  return current.split("-")[0] === candidate.split("-")[0]
}

export function LoginPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { setAuthenticatedSession } = useAuth()
  const showDevTestAccounts = import.meta.env.DEV && import.meta.env.VITE_SHOW_TEST_ACCOUNTS !== "false"
  const [error, setError] = useState("")
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof LoginSubmitFormValues, string>>>({})
  const [credentials, setCredentials] = useState<LoginSubmitFormValues>({
    email: "",
    password: "",
  })
  const [isSubmitting, setIsSubmitting] = useState(false)
  const loginSubmitSchema = useMemo(() => buildLoginSubmitSchema(t), [t])
  const currentLanguage = i18n.resolvedLanguage ?? i18n.language

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setFieldErrors({})

    const parsed = loginSubmitSchema.safeParse(credentials)
    if (!parsed.success) {
      const nextFieldErrors: Partial<Record<keyof LoginSubmitFormValues, string>> = {}
      for (const issue of parsed.error.issues) {
        const field = issue.path[0]
        if ((field === "email" || field === "password") && !nextFieldErrors[field]) {
          nextFieldErrors[field] = issue.message
        }
      }
      setFieldErrors(nextFieldErrors)
      return
    }

    setIsSubmitting(true)

    try {
      const response = await login(parsed.data.email, parsed.data.password)
      setAuthenticatedSession(response.access_token, response.refresh_token, response.user)
      navigate("/dashboard", { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("login.error.failed")
      setError(message)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="mp-fog-surface min-h-screen bg-background">
      <div className="grid min-h-screen lg:grid-cols-2">
        <section className="hidden border-r border-white/10 bg-white/[0.025] p-10 lg:flex lg:flex-col lg:justify-between">
          <div className="flex items-center gap-3">
            <MistyIslandMark className="size-14" markClassName="h-12 w-16" />
            <div>
              <p className="text-[11px] font-semibold tracking-[0.24em] text-muted-foreground uppercase">MistyPass</p>
              <p className="text-sm text-foreground">{t("login.hero.badge")}</p>
            </div>
          </div>

          <div className="space-y-6">
            <h1 className="max-w-xl text-5xl font-semibold tracking-[-0.05em]">{t("login.hero.title")}</h1>
            <p className="max-w-lg text-sm text-muted-foreground">{t("login.hero.description")}</p>
            <div className="grid max-w-md gap-2">
              <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-black/20 px-3 py-2 text-sm">
                <Building2Icon className="size-4 text-white/75" />
                {t("login.hero.pointTenant")}
              </div>
              <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-black/20 px-3 py-2 text-sm">
                <LockKeyholeIcon className="size-4 text-white/75" />
                {t("login.hero.pointSync")}
              </div>
            </div>
          </div>

          <p className="mp-kpi-note">{t("login.hero.footer")}</p>
        </section>

        <section className="flex items-center justify-center p-4 sm:p-8">
          <Card className="w-full max-w-md">
            <CardHeader className="space-y-2">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-2">
                  <CardTitle className="text-2xl">{t("login.form.title")}</CardTitle>
                  <CardDescription>{t("login.form.description")}</CardDescription>
                </div>
                <div className="flex items-center gap-1 rounded-md border bg-muted/30 p-1">
                  {languageOptions.map((item) => (
                    <button
                      key={item.code}
                      type="button"
                      onClick={() => {
                        void i18n.changeLanguage(item.code)
                      }}
                      className={`rounded px-2 py-1 text-xs ${
                        isLanguageActive(currentLanguage, item.code)
                          ? "bg-background text-foreground shadow-sm"
                          : "text-muted-foreground"
                      }`}
                    >
                      {t(item.labelKey)}
                    </button>
                  ))}
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <form onSubmit={onSubmit} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="email">{t("login.form.emailLabel")}</Label>
                  <Input
                    id="email"
                    type="email"
                    name="email"
                    value={credentials.email}
                    onChange={(event) => {
                      setCredentials((current) => ({ ...current, email: event.target.value }))
                    }}
                    placeholder={t("login.form.emailPlaceholder")}
                    autoComplete="email"
                    aria-invalid={Boolean(fieldErrors.email)}
                  />
                  {fieldErrors.email ? (
                    <p className="text-sm text-destructive">{fieldErrors.email}</p>
                  ) : null}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="password">{t("login.form.passwordLabel")}</Label>
                  <Input
                    id="password"
                    type="password"
                    name="password"
                    value={credentials.password}
                    onChange={(event) => {
                      setCredentials((current) => ({ ...current, password: event.target.value }))
                    }}
                    placeholder={t("login.form.passwordPlaceholder")}
                    autoComplete="current-password"
                    aria-invalid={Boolean(fieldErrors.password)}
                  />
                  {fieldErrors.password ? (
                    <p className="text-sm text-destructive">{fieldErrors.password}</p>
                  ) : null}
                </div>

                {error ? (
                  <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </div>
                ) : null}

                <Button type="submit" className="w-full" disabled={isSubmitting}>
                  {isSubmitting ? t("login.form.submitting") : t("login.form.submit")}
                  <ArrowRightIcon className="ml-1.5 size-4" />
                </Button>
              </form>

              {showDevTestAccounts ? <p className="mp-kpi-note">{t("login.form.devAccounts")}</p> : null}
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
