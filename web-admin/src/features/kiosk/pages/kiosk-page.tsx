import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"

import { Button } from "@/components/ui/button"
import {
  createGuest,
  getVisitorNDATemplate,
  listGuests,
  signGuestNDA,
  updateGuestStatus,
  type CurrentUser,
  type Guest,
} from "@/lib/api"

import { filterExpectedGuests, ndaStepNeeded } from "./kiosk-page-utils"

type KioskStep = "welcome" | "find" | "walkin" | "nda" | "done"

type KioskPageProps = {
  token: string
  viewer: CurrentUser
}

function SignaturePad({ onChange }: { onChange: (dataURL: string | null) => void }) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const drawingRef = useRef(false)
  const inkRef = useRef(false)

  const pointerPosition = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    if (!canvas) return { x: 0, y: 0 }
    const rect = canvas.getBoundingClientRect()
    return {
      x: ((event.clientX - rect.left) / rect.width) * canvas.width,
      y: ((event.clientY - rect.top) / rect.height) * canvas.height,
    }
  }

  const handlePointerDown = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext("2d")
    if (!canvas || !ctx) return
    try {
      canvas.setPointerCapture(event.pointerId)
    } catch {
      // Pointer capture is best-effort; drawing must not depend on it.
    }
    drawingRef.current = true
    const { x, y } = pointerPosition(event)
    ctx.lineWidth = 3
    ctx.lineCap = "round"
    ctx.strokeStyle = "#141510"
    ctx.beginPath()
    ctx.moveTo(x, y)
  }

  const handlePointerMove = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawingRef.current) return
    const ctx = canvasRef.current?.getContext("2d")
    if (!ctx) return
    const { x, y } = pointerPosition(event)
    ctx.lineTo(x, y)
    ctx.stroke()
    inkRef.current = true
  }

  const handlePointerUp = () => {
    if (!drawingRef.current) return
    drawingRef.current = false
    const canvas = canvasRef.current
    if (canvas && inkRef.current) {
      onChange(canvas.toDataURL("image/png"))
    }
  }

  const clear = useCallback(() => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext("2d")
    if (!canvas || !ctx) return
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    inkRef.current = false
    onChange(null)
  }, [onChange])

  return (
    <div className="space-y-2">
      <canvas
        ref={canvasRef}
        width={640}
        height={200}
        className="h-40 w-full touch-none rounded-lg border border-dashed border-input bg-white"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerLeave={handlePointerUp}
        aria-label={t("kiosk.signHere")}
      />
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{t("kiosk.signHere")}</span>
        <Button type="button" variant="ghost" size="sm" onClick={clear}>
          {t("kiosk.clearSignature")}
        </Button>
      </div>
    </div>
  )
}

export function KioskPage({ token, viewer }: KioskPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id

  const [step, setStep] = useState<KioskStep>("welcome")
  const [query, setQuery] = useState("")
  const [activeGuest, setActiveGuest] = useState<Guest | null>(null)
  const [signerName, setSignerName] = useState("")
  const [signature, setSignature] = useState<string | null>(null)
  const [walkIn, setWalkIn] = useState({ name: "", phone: "", host_name: "", company: "", purpose: "" })
  const [errorMessage, setErrorMessage] = useState("")

  const guestsQuery = useQuery({
    queryKey: ["kiosk-guests", tenantID],
    queryFn: () => listGuests(token, tenantID),
    enabled: Boolean(token && tenantID) && step === "find",
    refetchInterval: 30_000,
  })

  const ndaQuery = useQuery({
    queryKey: ["kiosk-nda-template", tenantID],
    queryFn: () => getVisitorNDATemplate(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const resetToWelcome = useCallback(() => {
    setStep("welcome")
    setQuery("")
    setActiveGuest(null)
    setSignerName("")
    setSignature(null)
    setWalkIn({ name: "", phone: "", host_name: "", company: "", purpose: "" })
    setErrorMessage("")
  }, [])

  useEffect(() => {
    if (step !== "done") return
    const timer = window.setTimeout(resetToWelcome, 8000)
    return () => window.clearTimeout(timer)
  }, [step, resetToWelcome])

  const checkInMutation = useMutation({
    mutationFn: (guest: Guest) => updateGuestStatus(token, guest.id, tenantID, "checked_in"),
    onSuccess: (guest) => {
      setActiveGuest(guest)
      setErrorMessage("")
      setStep("done")
      void queryClient.invalidateQueries({ queryKey: ["kiosk-guests", tenantID] })
      void queryClient.invalidateQueries({ queryKey: ["guests", tenantID] })
    },
    onError: () => setErrorMessage(t("kiosk.errorGeneric")),
  })

  const signMutation = useMutation({
    mutationFn: (guest: Guest) =>
      signGuestNDA(token, guest.id, {
        tenant_id: tenantID,
        signer_name: signerName.trim() || guest.name,
        signature_data_url: signature ?? "",
      }),
    onSuccess: (guest) => {
      setErrorMessage("")
      checkInMutation.mutate(guest)
    },
    onError: () => setErrorMessage(t("kiosk.errorGeneric")),
  })

  const walkInMutation = useMutation({
    mutationFn: () =>
      createGuest(token, {
        tenant_id: tenantID,
        name: walkIn.name.trim(),
        phone: walkIn.phone.trim(),
        host_name: walkIn.host_name.trim(),
        company: walkIn.company.trim() || undefined,
        purpose: walkIn.purpose.trim() || undefined,
        notify_host: true,
      }),
    onSuccess: (guest) => {
      setErrorMessage("")
      proceedAfterGuestChosen(guest)
    },
    onError: () => setErrorMessage(t("kiosk.errorGeneric")),
  })

  const ndaTemplate = ndaQuery.data
  const needsNDA = ndaStepNeeded(ndaTemplate)

  function proceedAfterGuestChosen(guest: Guest) {
    setActiveGuest(guest)
    setSignerName(guest.name)
    setSignature(null)
    if (needsNDA && !guest.nda_signed_at) {
      setStep("nda")
      return
    }
    checkInMutation.mutate(guest)
  }

  const busy = checkInMutation.isPending || signMutation.isPending || walkInMutation.isPending
  const expectedGuests = filterExpectedGuests(guestsQuery.data?.items ?? [], query)

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <header className="flex items-center justify-between border-b border-border px-6 py-4">
        <div>
          <div className="text-lg font-bold text-foreground">{t("kiosk.title")}</div>
          <div className="text-xs text-muted-foreground">{viewer.tenant_id}</div>
        </div>
        <Button variant="ghost" size="sm" onClick={() => navigate("/visitors")}>
          {t("kiosk.exit")}
        </Button>
      </header>

      <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-10">
        {errorMessage ? (
          <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {errorMessage}
          </div>
        ) : null}

        {step === "welcome" ? (
          <div className="space-y-8 text-center">
            <div>
              <h1 className="text-4xl font-extrabold tracking-tight text-foreground">{t("kiosk.welcome")}</h1>
              <p className="mt-3 text-base text-muted-foreground">{t("kiosk.welcomeHint")}</p>
            </div>
            <div className="flex flex-col gap-4">
              <Button size="lg" className="h-16 text-lg" onClick={() => setStep("find")}>
                {t("kiosk.startCheckIn")}
              </Button>
              <Button size="lg" variant="outline" className="h-16 text-lg" onClick={() => setStep("walkin")}>
                {t("kiosk.walkIn")}
              </Button>
            </div>
          </div>
        ) : null}

        {step === "find" ? (
          <div className="space-y-4">
            <h2 className="text-2xl font-bold text-foreground">{t("kiosk.expectedToday")}</h2>
            <input
              autoFocus
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("kiosk.searchPlaceholder")}
              className="h-12 w-full rounded-lg border border-input bg-background px-4 text-base"
            />
            <div className="max-h-80 space-y-2 overflow-y-auto">
              {guestsQuery.isLoading ? (
                <div className="py-6 text-center text-sm text-muted-foreground">{t("kiosk.loading")}</div>
              ) : expectedGuests.length === 0 ? (
                <div className="py-6 text-center text-sm text-muted-foreground">{t("kiosk.noExpected")}</div>
              ) : (
                expectedGuests.map((guest) => (
                  <button
                    key={guest.id}
                    type="button"
                    disabled={busy}
                    onClick={() => proceedAfterGuestChosen(guest)}
                    className="flex w-full items-center justify-between rounded-lg border border-border bg-card px-4 py-3 text-left hover:border-primary"
                  >
                    <span>
                      <span className="block text-base font-semibold text-foreground">{guest.name}</span>
                      <span className="block text-xs text-muted-foreground">
                        {t("kiosk.hostLabel")}: {guest.host_name}
                        {guest.company ? ` · ${guest.company}` : ""}
                      </span>
                    </span>
                    <span className="text-sm font-medium text-primary">{t("kiosk.checkIn")}</span>
                  </button>
                ))
              )}
            </div>
            <div className="flex justify-between">
              <Button variant="ghost" onClick={resetToWelcome}>{t("kiosk.back")}</Button>
              <Button variant="outline" onClick={() => setStep("walkin")}>{t("kiosk.walkIn")}</Button>
            </div>
          </div>
        ) : null}

        {step === "walkin" ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              if (!walkIn.name.trim() || !walkIn.phone.trim() || !walkIn.host_name.trim()) {
                setErrorMessage(t("kiosk.formRequired"))
                return
              }
              walkInMutation.mutate()
            }}
          >
            <h2 className="text-2xl font-bold text-foreground">{t("kiosk.walkInTitle")}</h2>
            <div className="grid gap-3">
              <input value={walkIn.name} onChange={(e) => setWalkIn({ ...walkIn, name: e.target.value })} placeholder={t("kiosk.fieldName")} className="h-12 rounded-lg border border-input bg-background px-4 text-base" />
              <input value={walkIn.phone} onChange={(e) => setWalkIn({ ...walkIn, phone: e.target.value })} placeholder={t("kiosk.fieldPhone")} className="h-12 rounded-lg border border-input bg-background px-4 text-base" />
              <input value={walkIn.host_name} onChange={(e) => setWalkIn({ ...walkIn, host_name: e.target.value })} placeholder={t("kiosk.fieldHost")} className="h-12 rounded-lg border border-input bg-background px-4 text-base" />
              <input value={walkIn.company} onChange={(e) => setWalkIn({ ...walkIn, company: e.target.value })} placeholder={t("kiosk.fieldCompany")} className="h-12 rounded-lg border border-input bg-background px-4 text-base" />
              <input value={walkIn.purpose} onChange={(e) => setWalkIn({ ...walkIn, purpose: e.target.value })} placeholder={t("kiosk.fieldPurpose")} className="h-12 rounded-lg border border-input bg-background px-4 text-base" />
            </div>
            <div className="flex justify-between">
              <Button type="button" variant="ghost" onClick={resetToWelcome}>{t("kiosk.back")}</Button>
              <Button type="submit" disabled={busy}>{busy ? t("kiosk.working") : t("kiosk.register")}</Button>
            </div>
          </form>
        ) : null}

        {step === "nda" && activeGuest ? (
          <div className="space-y-4">
            <h2 className="text-2xl font-bold text-foreground">{ndaTemplate?.title || t("kiosk.ndaTitle")}</h2>
            <div className="max-h-56 overflow-y-auto whitespace-pre-wrap rounded-lg border border-border bg-card px-4 py-3 text-sm text-foreground">
              {ndaTemplate?.body}
            </div>
            <input
              value={signerName}
              onChange={(event) => setSignerName(event.target.value)}
              placeholder={t("kiosk.signerName")}
              className="h-12 w-full rounded-lg border border-input bg-background px-4 text-base"
            />
            <SignaturePad onChange={setSignature} />
            {ndaTemplate?.required ? (
              <p className="text-xs text-muted-foreground">{t("kiosk.signRequiredNote")}</p>
            ) : null}
            <div className="flex justify-between">
              <Button variant="ghost" onClick={resetToWelcome}>{t("kiosk.back")}</Button>
              <div className="flex gap-2">
                {!ndaTemplate?.required ? (
                  <Button variant="outline" disabled={busy} onClick={() => checkInMutation.mutate(activeGuest)}>
                    {t("kiosk.skip")}
                  </Button>
                ) : null}
                <Button
                  disabled={busy || !signature || !signerName.trim()}
                  onClick={() => signMutation.mutate(activeGuest)}
                >
                  {busy ? t("kiosk.working") : t("kiosk.agreeAndSign")}
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        {step === "done" && activeGuest ? (
          <div className="space-y-6 text-center">
            <div className="text-5xl">✅</div>
            <div>
              <h2 className="text-3xl font-extrabold text-foreground">
                {t("kiosk.welcomeName", { name: activeGuest.name })}
              </h2>
              <p className="mt-2 text-base text-muted-foreground">{t("kiosk.checkInSuccess")}</p>
              {activeGuest.notify_host ? (
                <p className="mt-1 text-sm text-muted-foreground">{t("kiosk.hostNotified", { host: activeGuest.host_name })}</p>
              ) : null}
            </div>
            <Button size="lg" variant="outline" onClick={resetToWelcome}>
              {t("kiosk.newCheckIn")}
            </Button>
          </div>
        ) : null}
      </main>
    </div>
  )
}
