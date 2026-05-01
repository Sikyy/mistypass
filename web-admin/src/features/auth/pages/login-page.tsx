import type { TFunction } from "i18next"
import { ArrowRightIcon, Building2Icon, FingerprintIcon, GlobeIcon, LockKeyholeIcon } from "lucide-react"
import { type FormEvent, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { useAuth } from "@/context/auth-context"
import { confirmPasswordReset, login, requestPasswordReset, userSignUp, webAuthnLoginBegin, webAuthnLoginFinish, webAuthnLoginFinishMFA } from "@/lib/api"

const languageOptions = [
  { code: "zh-CN", labelKey: "common.language.zh" },
  { code: "en-US", labelKey: "common.language.en" },
  { code: "id-ID", labelKey: "common.language.id" },
] as const

// Demo accounts are only used in dev mode (DEV is a compile-time constant;
// Vite tree-shakes this in production builds).
const demoAccounts = import.meta.env.DEV
  ? ([
      {
        label: "Organization Admin",
        scope: "Mistyislet organization",
        email: "organization.admin@mistypass.local",
        password: "admin123",
      },
      {
        label: "Place Admin",
        scope: "Sudirman Hub place",
        email: "place.admin.sudirman@mistypass.local",
        password: "admin123",
      },
    ] as const)
  : ([] as const)

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

function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/")
  const padded = base64.padEnd(base64.length + (4 - (base64.length % 4)) % 4, "=")
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ""
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}

function serializePublicKeyCredential(credential: PublicKeyCredential): any {
  const response = credential.response as AuthenticatorAssertionResponse
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      authenticatorData: bufferToBase64url(response.authenticatorData),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : undefined,
    },
  }
}

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
  const [isPasskeySubmitting, setIsPasskeySubmitting] = useState(false)
  const [passkeyMFA, setPasskeyMFA] = useState<{ webauthnToken: string } | null>(null)
  const [mfaCode, setMfaCode] = useState("")
  const [showSignUp, setShowSignUp] = useState(false)
  const [signUpName, setSignUpName] = useState("")
  const [resetStep, setResetStep] = useState<"none" | "requested" | "confirm">("none")
  const [resetToken, setResetToken] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [resetMessage, setResetMessage] = useState("")
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
      navigate("/home", { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : t("login.error.failed")
      setError(message)
    } finally {
      setIsSubmitting(false)
    }
  }

  async function onPasskeyLogin() {
    const email = credentials.email.trim()
    if (!email) {
      setFieldErrors({ email: t("login.validation.emailRequired") })
      return
    }
    setError("")
    setFieldErrors({})
    setPasskeyMFA(null)
    setIsPasskeySubmitting(true)
    try {
      const optionsRaw = await webAuthnLoginBegin(email)
      const pk = (optionsRaw as any).publicKey
      const options: CredentialRequestOptions = {
        publicKey: {
          ...pk,
          challenge: base64urlToBuffer(pk.challenge),
          allowCredentials: pk.allowCredentials?.map((c: any) => ({
            ...c,
            id: base64urlToBuffer(c.id),
          })),
        },
      }
      const credential = await navigator.credentials.get(options)
      if (!credential) throw new Error("Authentication cancelled")
      const serialized = serializePublicKeyCredential(credential as PublicKeyCredential)
      const result = await webAuthnLoginFinish(email, serialized)
      if (result.mfa_required) {
        setPasskeyMFA({ webauthnToken: result.webauthn_token })
        setMfaCode("")
        return
      }
      setAuthenticatedSession(result.access_token, result.refresh_token, result.user)
      navigate("/home", { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : "Passkey sign-in failed"
      setError(message)
    } finally {
      setIsPasskeySubmitting(false)
    }
  }

  async function onPasskeyMFASubmit(event: FormEvent) {
    event.preventDefault()
    if (!passkeyMFA || !mfaCode.trim()) return
    setError("")
    setIsPasskeySubmitting(true)
    try {
      const response = await webAuthnLoginFinishMFA(passkeyMFA.webauthnToken, mfaCode.trim())
      setAuthenticatedSession(response.access_token, response.refresh_token, response.user)
      navigate("/home", { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : "MFA verification failed"
      setError(message)
    } finally {
      setIsPasskeySubmitting(false)
    }
  }

  return (
    <div className="mp-fog-surface min-h-screen bg-background">
      <div className="grid min-h-screen lg:grid-cols-2">
        <section className="hidden border-r border-white/10 bg-white/[0.025] p-10 lg:flex lg:flex-col lg:justify-between">
          <div className="flex items-center gap-3">
            <MistyIslandMark className="size-14" markClassName="h-12 w-16" />
            <div>
              <p className="text-[11px] font-semibold tracking-[0.24em] text-muted-foreground uppercase">Mistyislet</p>
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
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm" className="gap-1.5 text-xs">
                      <GlobeIcon className="size-3.5" />
                      {languageOptions.find((l) => isLanguageActive(currentLanguage, l.code))?.labelKey
                        ? t(languageOptions.find((l) => isLanguageActive(currentLanguage, l.code))!.labelKey)
                        : "Language"}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-32">
                    {languageOptions.map((item) => (
                      <DropdownMenuItem
                        key={item.code}
                        className="cursor-pointer text-sm"
                        onSelect={() => void i18n.changeLanguage(item.code)}
                      >
                        <span className="flex-1">{t(item.labelKey)}</span>
                        {isLanguageActive(currentLanguage, item.code) ? (
                          <span className="size-2 rounded-full bg-primary" />
                        ) : null}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
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
                  <button
                    type="button"
                    className="text-xs text-muted-foreground hover:text-foreground hover:underline"
                    onClick={async () => {
                      const email = credentials.email.trim()
                      if (!email) {
                        setFieldErrors({ email: t("login.validation.emailRequired") })
                        return
                      }
                      setError("")
                      setResetMessage("")
                      try {
                        const res = await requestPasswordReset(email)
                        if (res.reset_token) {
                          setResetToken(res.reset_token)
                          setResetStep("confirm")
                        } else {
                          setResetStep("requested")
                          setResetMessage("If an account exists with that email, a reset link has been sent.")
                        }
                      } catch {
                        setResetStep("requested")
                        setResetMessage("If an account exists with that email, a reset link has been sent.")
                      }
                    }}
                  >
                    Forgot password?
                  </button>
                </div>

                {error ? (
                  <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </div>
                ) : null}

                {resetMessage ? (
                  <div className="rounded-lg border border-primary/20 bg-primary/5 px-3 py-2 text-sm">
                    {resetMessage}
                  </div>
                ) : null}

                {resetStep === "confirm" ? (
                  <div className="space-y-3 rounded-lg border border-primary/20 bg-primary/5 p-4">
                    <p className="text-sm font-medium">Set your new password</p>
                    <Input
                      type="password"
                      placeholder="New password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      autoComplete="new-password"
                    />
                    <div className="flex gap-2">
                      <Button
                        type="button"
                        className="flex-1"
                        disabled={isSubmitting || newPassword.length < 8}
                        onClick={async () => {
                          setError("")
                          setIsSubmitting(true)
                          try {
                            await confirmPasswordReset(resetToken, newPassword)
                            setResetStep("none")
                            setResetMessage("Password reset successful. You can now log in.")
                            setNewPassword("")
                            setResetToken("")
                          } catch (err) {
                            setError(err instanceof Error ? err.message : "Reset failed")
                          } finally {
                            setIsSubmitting(false)
                          }
                        }}
                      >
                        Reset password
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => { setResetStep("none"); setResetToken(""); setNewPassword(""); setResetMessage("") }}
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                ) : null}

                <Button type="submit" className="w-full" disabled={isSubmitting || isPasskeySubmitting}>
                  {isSubmitting ? t("login.form.submitting") : t("login.form.submit")}
                  <ArrowRightIcon className="ml-1.5 size-4" />
                </Button>
              </form>

              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-card px-2 text-muted-foreground">or</span>
                </div>
              </div>

              <Button
                type="button"
                variant="outline"
                className="w-full"
                disabled={isSubmitting || isPasskeySubmitting}
                onClick={onPasskeyLogin}
              >
                <FingerprintIcon className="mr-2 size-4" />
                {isPasskeySubmitting ? "Authenticating..." : "Sign in with passkey"}
              </Button>

              {passkeyMFA ? (
                <form onSubmit={onPasskeyMFASubmit} className="space-y-3 rounded-lg border border-primary/20 bg-primary/5 p-4">
                  <p className="text-sm font-medium">Passkey verified. Enter your MFA code to continue.</p>
                  <div className="flex gap-2">
                    <Input
                      type="text"
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      placeholder="6-digit code"
                      maxLength={6}
                      value={mfaCode}
                      onChange={(e) => setMfaCode(e.target.value)}
                      className="flex-1"
                      autoFocus
                    />
                    <Button type="submit" disabled={isPasskeySubmitting || mfaCode.trim().length < 6}>
                      {isPasskeySubmitting ? "Verifying..." : "Verify"}
                    </Button>
                  </div>
                  <button
                    type="button"
                    className="text-xs text-muted-foreground hover:underline"
                    onClick={() => { setPasskeyMFA(null); setMfaCode(""); setError("") }}
                  >
                    Cancel
                  </button>
                </form>
              ) : null}

              <div className="text-center">
                <button type="button" className="text-xs text-muted-foreground hover:text-foreground hover:underline" onClick={() => setShowSignUp(!showSignUp)}>
                  {showSignUp ? "Back to login" : "Create an account"}
                </button>
              </div>

              {showSignUp ? (
                <div className="space-y-3 rounded-lg border border-primary/20 bg-primary/5 p-4">
                  <p className="text-sm font-medium">Create a new account</p>
                  <Input type="text" placeholder="Name" value={signUpName} onChange={(e) => setSignUpName(e.target.value)} autoComplete="name" />
                  <Input type="email" placeholder="Email" value={credentials.email} onChange={(e) => setCredentials((c) => ({ ...c, email: e.target.value }))} autoComplete="email" />
                  <Input type="password" placeholder="Password (min 8 chars, upper+lower+digit)" value={credentials.password} onChange={(e) => setCredentials((c) => ({ ...c, password: e.target.value }))} autoComplete="new-password" />
                  <Button
                    type="button"
                    className="w-full"
                    disabled={isSubmitting || !credentials.email.trim() || !credentials.password.trim()}
                    onClick={async () => {
                      setError("")
                      setIsSubmitting(true)
                      try {
                        await userSignUp({ name: signUpName, email: credentials.email, password: credentials.password })
                        setShowSignUp(false)
                        setResetMessage("Account created. You can now log in.")
                      } catch (err) {
                        setError(err instanceof Error ? err.message : "Sign up failed")
                      } finally {
                        setIsSubmitting(false)
                      }
                    }}
                  >
                    {isSubmitting ? "Creating..." : "Sign Up"}
                  </Button>
                </div>
              ) : null}

              {showDevTestAccounts ? (
                <div className="space-y-2">
                  <p className="mp-kpi-note">{t("login.form.devAccounts")}</p>
                  <div className="grid gap-2">
                    {demoAccounts.map((account) => (
                      <button
                        key={account.email}
                        type="button"
                        onClick={() => {
                          setCredentials({ email: account.email, password: account.password })
                        }}
                        className="flex items-center justify-between rounded-lg border border-white/10 bg-white/[0.045] px-3 py-2 text-left text-sm transition-colors hover:bg-white/[0.075]"
                      >
                        <span>
                          <span className="block font-semibold text-foreground">{account.label}</span>
                          <span className="mt-0.5 block text-xs text-muted-foreground">{account.email}</span>
                        </span>
                        <span className="text-xs text-muted-foreground">{account.scope}</span>
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
