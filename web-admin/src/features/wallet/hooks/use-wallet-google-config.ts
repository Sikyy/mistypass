import { useState } from "react"
import { useTranslation } from "react-i18next"

import {
  APIError,
  getWalletGoogleConfig,
  upsertWalletGoogleConfig,
  validateWalletGoogleConfig,
  type WalletGoogleConfig,
  type WalletGoogleConfigPayload,
  type WalletGoogleConfigValidation,
  type WalletGoogleConfigValidationPayload,
} from "@/lib/api"

type UseWalletGoogleConfigParams = {
  token: string
  tenantID: string
}

export function useWalletGoogleConfig({
  token,
  tenantID,
}: UseWalletGoogleConfigParams) {
  const { t } = useTranslation()

  const [config, setConfig] = useState<WalletGoogleConfig | null>(null)
  const [validation, setValidation] =
    useState<WalletGoogleConfigValidation | null>(null)
  const [loadingConfig, setLoadingConfig] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)
  const [validatingConfig, setValidatingConfig] = useState(false)
  const [summary, setSummary] = useState("")
  const [error, setError] = useState("")

  async function loadGoogleConfig(nextTenantID: string) {
    const effectiveTenantID = nextTenantID.trim()
    if (!effectiveTenantID) {
      resetGoogleConfig()
      return
    }

    setLoadingConfig(true)
    setError("")
    try {
      const data = await getWalletGoogleConfig(token, effectiveTenantID)
      setConfig(data)
      setValidation(null)
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        setConfig(null)
        setValidation(null)
        return
      }
      const message =
        err instanceof Error
          ? err.message
          : t("walletPage.errors.loadGoogleConfigFailed")
      setError(message)
    } finally {
      setLoadingConfig(false)
    }
  }

  function resetGoogleConfig() {
    setConfig(null)
    setValidation(null)
    setSummary("")
    setError("")
  }

  async function saveGoogleConfig(
    payload: Omit<WalletGoogleConfigPayload, "tenant_id" | "actor">
  ) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setSavingConfig(true)
    setSummary("")
    setError("")
    try {
      const updated = await upsertWalletGoogleConfig(token, {
        ...payload,
        tenant_id: nextTenantID,
        actor: "web_admin.wallet.google_config",
      })
      setConfig(updated)
      setValidation(null)
      setSummary(
        t("walletPage.summaries.googleConfigSaved", {
          issuerID: updated.issuer_id,
          status: updated.status,
        })
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("walletPage.errors.saveGoogleConfigFailed")
      setError(message)
    } finally {
      setSavingConfig(false)
    }
  }

  async function validateGoogleConfig(
    payload: Omit<WalletGoogleConfigValidationPayload, "tenant_id">
  ) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setValidatingConfig(true)
    setSummary("")
    setError("")
    try {
      const result = await validateWalletGoogleConfig(token, {
        ...payload,
        tenant_id: nextTenantID,
      })
      setValidation(result)
      setSummary(
        result.valid
          ? t("walletPage.summaries.googleConfigValidationPassed")
          : t("walletPage.summaries.googleConfigValidationFailed")
      )
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : t("walletPage.errors.validateGoogleConfigFailed")
      setError(message)
    } finally {
      setValidatingConfig(false)
    }
  }

  return {
    config,
    validation,
    loadingConfig,
    savingConfig,
    validatingConfig,
    summary,
    error,
    setError,
    loadGoogleConfig,
    resetGoogleConfig,
    saveGoogleConfig,
    validateGoogleConfig,
  }
}
