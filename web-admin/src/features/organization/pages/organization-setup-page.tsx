import i18next from "i18next"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  AlertCircleIcon,
  BarChart3Icon,
  BellIcon,
  ChevronDownIcon,
  CloudIcon,
  CreditCardIcon,
  FingerprintIcon,
  KeyRoundIcon,
  PlusIcon,
  ServerIcon,
  ShieldCheckIcon,
  Trash2Icon,
  UsersIcon,
} from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import {
  FormField,
  PageFrame,
  SettingsPanel,
  SettingToggleRows,
  StatusDot,
  ToggleSwitch,
} from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  createAlertPolicy,
  createIntegration,
  deleteAlertPolicy,
  deleteIntegration,
  disableOrganization,
  exportOrganizationAudit,
  getIntegration,
  getOrganizationSettings,
  listAlertPolicies,
  listIntegrations,
  previewAlertPolicyCondition,
  rotateOrganizationWebhooks,
  updateIntegration,
  updateAlertPolicy,
  updateOrganizationSettings,
  type AlertPolicy,
  type CurrentUser,
  type Integration,
  type OrganizationSettings,
} from "@/lib/api"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

type IntegrationRow = [string, string, string, "success" | "warning" | "info"]
type AlertPolicyRow = [string, string, string, "success" | "warning" | "info"]
type AlertPolicyCategory = "enterprise_sync_worker" | "wallet_jobs" | "custom"
type IntegrationDraft = {
  tenant_id: string
  type: "identity_provider" | "hris"
  provider: string
  status: string
  sync_mode: string
  credential_ref: string
  webhook_secret_ref: string
  issuer_url: string
  client_id: string
  auth_url: string
}
type AlertPolicyDraft = {
  name: string
  description: string
  trigger: string
  severity: string
  condition_expression: string
  enabled: boolean
  threshold: string
  window_seconds: string
  cooldown_seconds: string
  email: boolean
  whatsapp: boolean
  receiver_groups: string
}

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
  return visibleIntegrationsForTab(integrations, activeTab, isSsoScim).map((integration) => [
    integration.name,
    integrationStatusLabel(integration),
    integration.description,
    integrationStatus(integration),
  ])
}

function visibleIntegrationsForTab(integrations: Integration[], activeTab: string, isSsoScim: boolean): Integration[] {
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

  return visible
}

function defaultIntegrationDraft(tenantID: string): IntegrationDraft {
  return {
    tenant_id: tenantID,
    type: "hris",
    provider: "talenta",
    status: "active",
    sync_mode: "hybrid",
    credential_ref: "",
    webhook_secret_ref: "",
    issuer_url: "",
    client_id: "",
    auth_url: "",
  }
}

function integrationDraftFromIntegration(integration: Integration, tenantID: string): IntegrationDraft {
  return {
    ...defaultIntegrationDraft(tenantID || integration.tenant_id),
    tenant_id: integration.tenant_id || tenantID,
    type: integration.type === "identity_provider" ? "identity_provider" : "hris",
    provider: integration.provider,
    status: integration.status,
    sync_mode: integration.sync_mode || (integration.type === "identity_provider" ? "jit" : "hybrid"),
  }
}

function formatPolicySeconds(seconds: number) {
  if (seconds >= 3600 && seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds >= 60 && seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}

function alertPolicyStatusLabel(policy: AlertPolicy) {
  return policy.enabled && policy.status === "active" ? "Enabled" : "Review"
}

function alertPolicyTone(policy: AlertPolicy): AlertPolicyRow[3] {
  return policy.enabled && policy.status === "active" ? "success" : "warning"
}

function liveAlertPolicyRows(policies: AlertPolicy[], activeTab: string): AlertPolicyRow[] {
  return policies.map((policy) => {
    if (activeTab === "Notifications") {
      const channels = [
        policy.channels.email ? "Email" : "",
        policy.channels.whatsapp ? "WhatsApp" : "",
      ].filter(Boolean)
      const receivers = policy.receiver_groups?.length ? policy.receiver_groups.join(", ") : "security"
      return [
        `${policy.name} notifications`,
        alertPolicyStatusLabel(policy),
        `${channels.length > 0 ? channels.join(" + ") : "No channels"} to ${receivers}`,
        alertPolicyTone(policy),
      ]
    }
    if (activeTab === "Escalation") {
      return [
        `${policy.name} escalation`,
        alertPolicyStatusLabel(policy),
        `Threshold ${policy.threshold}; cooldown ${formatPolicySeconds(policy.cooldown_seconds)} after each dispatch`,
        alertPolicyTone(policy),
      ]
    }
    return [
      policy.name,
      alertPolicyStatusLabel(policy),
      `${policy.description} Trigger ${policy.trigger}; window ${formatPolicySeconds(policy.window_seconds)}.`,
      alertPolicyTone(policy),
    ]
  })
}

function receiverGroupsText(policy: AlertPolicy) {
  return policy.receiver_groups?.length ? policy.receiver_groups.join(", ") : "security"
}

function alertPolicyDraftFromPolicy(policy: AlertPolicy): AlertPolicyDraft {
  return {
    name: policy.name,
    description: policy.description,
    trigger: policy.trigger,
    severity: policy.severity,
    condition_expression: policy.condition_expression ?? "",
    enabled: policy.enabled,
    threshold: String(policy.threshold),
    window_seconds: String(policy.window_seconds),
    cooldown_seconds: String(policy.cooldown_seconds),
    email: policy.channels.email,
    whatsapp: policy.channels.whatsapp,
    receiver_groups: receiverGroupsText(policy),
  }
}

function defaultAlertPolicyDraft(category: AlertPolicyCategory): AlertPolicyDraft {
  const isCustom = category === "custom"
  return {
    name: isCustom ? "After-hours access review" : "",
    description: isCustom ? "Escalate access events that match a custom condition." : "",
    trigger: isCustom ? "access_denied_after_hours" : category === "wallet_jobs" ? "wallet_job_dlq_threshold" : "worker_failure_threshold",
    severity: isCustom ? "high" : "high",
    condition_expression: isCustom ? "event.type == 'access_denied' && event.hour >= 18" : "",
    enabled: true,
    threshold: category === "wallet_jobs" ? "20" : "3",
    window_seconds: "900",
    cooldown_seconds: "900",
    email: true,
    whatsapp: false,
    receiver_groups: "security",
  }
}

function splitReceiverGroups(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseAlertPolicyInteger(value: string, label: string, min: number) {
  const normalized = value.trim()
  const parsed = Number.parseInt(normalized, 10)
  if (!/^\d+$/.test(normalized)) {
    throw new Error(`${label} must be ${min === 0 ? "0 or greater" : "1 or greater"}`)
  }
  if (!Number.isFinite(parsed) || parsed < min) {
    throw new Error(`${label} must be ${min === 0 ? "0 or greater" : "1 or greater"}`)
  }
  return parsed
}

function isAlertPolicyDraftDirty(policy: AlertPolicy, draft: AlertPolicyDraft | undefined) {
  if (!draft) {
    return false
  }
  const currentReceiverGroups = policy.receiver_groups?.length ? policy.receiver_groups : ["security"]
  return (
    draft.enabled !== policy.enabled ||
    draft.threshold.trim() !== String(policy.threshold) ||
    draft.window_seconds.trim() !== String(policy.window_seconds) ||
    draft.cooldown_seconds.trim() !== String(policy.cooldown_seconds) ||
    draft.email !== policy.channels.email ||
    draft.whatsapp !== policy.channels.whatsapp ||
    splitReceiverGroups(draft.receiver_groups).join(",") !== currentReceiverGroups.join(",") ||
    (policy.category === "custom" &&
      (draft.name.trim() !== policy.name ||
        draft.description.trim() !== policy.description ||
        draft.trigger.trim() !== policy.trigger ||
        draft.severity.trim() !== policy.severity ||
        draft.condition_expression.trim() !== (policy.condition_expression ?? "")))
  )
}

function alertPolicyDraftStatusLabel(draft: AlertPolicyDraft) {
  return draft.enabled ? "Enabled" : "Review"
}

function alertPolicyDraftTone(draft: AlertPolicyDraft): AlertPolicyRow[3] {
  return draft.enabled ? "success" : "warning"
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
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const normalized = title.toLowerCase()
  const isCreatePlace = normalized.includes("create")
  const isAlertPolicies = normalized.includes("alert")
  const isIntegrations = normalized.includes("integrations")
  const isBilling = normalized.includes("billing")
  const isSettings = normalized.includes("settings")
  const isSsoScim = normalized.includes("sso")
  const tabs = isAlertPolicies
    ? [t("kisi.orgSetup.policies"), t("kisi.orgSetup.notifications"), t("kisi.orgSetup.escalation")]
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
  const [alertPolicyDrafts, setAlertPolicyDrafts] = useState<Record<string, AlertPolicyDraft>>({})
  const [alertPolicyError, setAlertPolicyError] = useState("")
  const [createAlertPolicyOpen, setCreateAlertPolicyOpen] = useState(false)
  const [createAlertPolicyTenantID, setCreateAlertPolicyTenantID] = useState("")
  const [createAlertPolicyCategory, setCreateAlertPolicyCategory] = useState<AlertPolicyCategory>("enterprise_sync_worker")
  const [createAlertPolicyDraft, setCreateAlertPolicyDraft] = useState<AlertPolicyDraft>(() => defaultAlertPolicyDraft("enterprise_sync_worker"))
  const [alertPolicyConditionPreview, setAlertPolicyConditionPreview] = useState<{ matched: boolean; condition_expression: string } | null>(null)
  const [deleteAlertPolicyTarget, setDeleteAlertPolicyTarget] = useState<AlertPolicy | null>(null)
  const tenantID = isPlatformViewer(viewer) ? undefined : getViewerTenantID(viewer)
  const defaultIntegrationTenantID = tenantID || getViewerTenantID(viewer)
  const [integrationSheetOpen, setIntegrationSheetOpen] = useState(false)
  const [editingIntegrationID, setEditingIntegrationID] = useState("")
  const [integrationDraft, setIntegrationDraft] = useState<IntegrationDraft>(() => defaultIntegrationDraft(defaultIntegrationTenantID))
  const [integrationError, setIntegrationError] = useState("")
  const [deleteIntegrationTarget, setDeleteIntegrationTarget] = useState<Integration | null>(null)
  const integrationsQuery = useQuery({
    queryKey: ["reference-integrations", viewer.id, viewer.role, tenantID ?? "platform"],
    queryFn: () => listIntegrations(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    enabled: isIntegrations || isSsoScim,
    staleTime: 30 * 1000,
  })
  const integrationDetailQuery = useQuery({
    queryKey: ["reference-integration-detail", editingIntegrationID, tenantID ?? defaultIntegrationTenantID],
    queryFn: () => getIntegration(token, editingIntegrationID, tenantID || defaultIntegrationTenantID),
    enabled: Boolean((isIntegrations || isSsoScim) && integrationSheetOpen && editingIntegrationID),
  })
  const alertPoliciesQuery = useQuery({
    queryKey: ["reference-alert-policies", viewer.id, viewer.role, tenantID ?? "platform"],
    queryFn: () => listAlertPolicies(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    enabled: isAlertPolicies,
    staleTime: 30 * 1000,
  })
  const liveAlertPolicies = alertPoliciesQuery.data ?? []
  const defaultAlertPolicyTenantID = tenantID || liveAlertPolicies[0]?.tenant_id || getViewerTenantID(viewer)

  useEffect(() => {
    if (!isAlertPolicies) {
      return
    }
    const nextDrafts = Object.fromEntries((alertPoliciesQuery.data ?? []).map((policy) => [policy.id, alertPolicyDraftFromPolicy(policy)]))
    setAlertPolicyDrafts(nextDrafts)
  }, [isAlertPolicies, alertPoliciesQuery.data])

  useEffect(() => {
    if (!integrationSheetOpen || !editingIntegrationID || !integrationDetailQuery.data) {
      return
    }
    setIntegrationDraft(integrationDraftFromIntegration(integrationDetailQuery.data, defaultIntegrationTenantID))
  }, [defaultIntegrationTenantID, editingIntegrationID, integrationDetailQuery.data, integrationSheetOpen])

  function updateAlertPolicyDraft(policyID: string, patch: Partial<AlertPolicyDraft>) {
    setAlertPolicyDrafts((current) => ({
      ...current,
      [policyID]: {
        ...(current[policyID] ?? {
          name: "",
          description: "",
          trigger: "",
          severity: "high",
          condition_expression: "",
          enabled: false,
          threshold: "",
          window_seconds: "",
          cooldown_seconds: "",
          email: false,
          whatsapp: false,
          receiver_groups: "",
        }),
        ...patch,
      },
    }))
  }

  function updateIntegrationDraft(patch: Partial<IntegrationDraft>) {
    setIntegrationDraft((current) => ({ ...current, ...patch }))
  }

  function openCreateIntegrationSheet() {
    setEditingIntegrationID("")
    setIntegrationDraft(defaultIntegrationDraft(defaultIntegrationTenantID))
    setIntegrationError("")
    setIntegrationSheetOpen(true)
  }

  function openEditIntegrationSheet(integration: Integration) {
    setEditingIntegrationID(integration.id)
    setIntegrationDraft(integrationDraftFromIntegration(integration, defaultIntegrationTenantID))
    setIntegrationError("")
    setIntegrationSheetOpen(true)
  }

  const alertPoliciesDirty = liveAlertPolicies.some((policy) => isAlertPolicyDraftDirty(policy, alertPolicyDrafts[policy.id]))
  const saveAlertPoliciesMutation = useMutation({
    mutationFn: async () => {
      const dirtyPolicies = liveAlertPolicies.filter((policy) => isAlertPolicyDraftDirty(policy, alertPolicyDrafts[policy.id]))
      return Promise.all(
        dirtyPolicies.map((policy) => {
          const draft = alertPolicyDrafts[policy.id] ?? alertPolicyDraftFromPolicy(policy)
          if (draft.enabled && !draft.email && !draft.whatsapp) {
            throw new Error(`${policy.name} needs at least one notification channel`)
          }
          return updateAlertPolicy(token, policy.id, {
            tenant_id: policy.tenant_id,
            name: policy.category === "custom" ? draft.name.trim() : undefined,
            description: policy.category === "custom" ? draft.description.trim() : undefined,
            trigger: policy.category === "custom" ? draft.trigger.trim() : undefined,
            severity: policy.category === "custom" ? draft.severity.trim() : undefined,
            condition_expression: policy.category === "custom" ? draft.condition_expression.trim() : undefined,
            enabled: draft.enabled,
            threshold: parseAlertPolicyInteger(draft.threshold, "Threshold", 1),
            window_seconds: parseAlertPolicyInteger(draft.window_seconds, "Window seconds", 1),
            cooldown_seconds: parseAlertPolicyInteger(draft.cooldown_seconds, "Cooldown seconds", 0),
            channels: {
              email: draft.email,
              whatsapp: draft.whatsapp,
            },
            receiver_groups: splitReceiverGroups(draft.receiver_groups),
            actor: viewer.email,
          })
        })
      )
    },
    onSuccess: async () => {
      setAlertPolicyError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-alert-policies"] })
    },
    onError: (error) => setAlertPolicyError(error instanceof Error ? error.message : i18next.t("kisi.orgSetup.description")),
  })

  const createAlertPolicyMutation = useMutation({
    mutationFn: () => {
      const nextTenantID = createAlertPolicyTenantID.trim() || defaultAlertPolicyTenantID
      if (!nextTenantID) {
        throw new Error("tenant_id is required")
      }
      if (createAlertPolicyDraft.enabled && !createAlertPolicyDraft.email && !createAlertPolicyDraft.whatsapp) {
        throw new Error(i18next.t("kisi.orgSetup.description"))
      }
      if (createAlertPolicyCategory === "custom" && !createAlertPolicyDraft.trigger.trim()) {
        throw new Error("Custom policy trigger is required")
      }
      return createAlertPolicy(token, {
        tenant_id: nextTenantID,
        category: createAlertPolicyCategory,
        name: createAlertPolicyCategory === "custom" ? createAlertPolicyDraft.name.trim() : undefined,
        description: createAlertPolicyCategory === "custom" ? createAlertPolicyDraft.description.trim() : undefined,
        trigger: createAlertPolicyDraft.trigger.trim() || undefined,
        severity: createAlertPolicyDraft.severity.trim() || undefined,
        condition_expression: createAlertPolicyCategory === "custom" ? createAlertPolicyDraft.condition_expression.trim() : undefined,
        enabled: createAlertPolicyDraft.enabled,
        threshold: parseAlertPolicyInteger(createAlertPolicyDraft.threshold, "Threshold", 1),
        window_seconds: parseAlertPolicyInteger(createAlertPolicyDraft.window_seconds, "Window seconds", 1),
        cooldown_seconds: parseAlertPolicyInteger(createAlertPolicyDraft.cooldown_seconds, "Cooldown seconds", 0),
        channels: {
          email: createAlertPolicyDraft.email,
          whatsapp: createAlertPolicyDraft.whatsapp,
        },
        receiver_groups: splitReceiverGroups(createAlertPolicyDraft.receiver_groups),
        actor: viewer.email,
      })
    },
    onSuccess: async () => {
      setCreateAlertPolicyOpen(false)
      setCreateAlertPolicyTenantID("")
      setCreateAlertPolicyCategory("enterprise_sync_worker")
      setCreateAlertPolicyDraft(defaultAlertPolicyDraft("enterprise_sync_worker"))
      setAlertPolicyError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-alert-policies"] })
    },
    onError: (error) => setAlertPolicyError(error instanceof Error ? error.message : i18next.t("kisi.orgSetup.description")),
  })

  const previewAlertPolicyConditionMutation = useMutation({
    mutationFn: () => {
      const nextTenantID = createAlertPolicyTenantID.trim() || defaultAlertPolicyTenantID
      if (!nextTenantID) {
        throw new Error("Tenant ID is required")
      }
      if (createAlertPolicyCategory !== "custom" || !createAlertPolicyDraft.condition_expression.trim()) {
        throw new Error("Condition expression is required")
      }
      return previewAlertPolicyCondition(token, {
        tenant_id: nextTenantID,
        condition_expression: createAlertPolicyDraft.condition_expression.trim(),
        event: {
          type: "access_denied",
          result: "denied",
          hour: 20,
          trigger: createAlertPolicyDraft.trigger.trim() || "access_denied_after_hours",
          source: "preview",
        },
      })
    },
    onSuccess: (preview) => {
      setAlertPolicyConditionPreview({
        matched: preview.matched,
        condition_expression: preview.condition_expression,
      })
      setAlertPolicyError("")
    },
    onError: (error) => {
      setAlertPolicyConditionPreview(null)
      setAlertPolicyError(error instanceof Error ? error.message : "Condition preview failed")
    },
  })

  const deleteAlertPolicyMutation = useMutation({
    mutationFn: (policyID: string) => deleteAlertPolicy(token, policyID),
    onSuccess: async () => {
      setDeleteAlertPolicyTarget(null)
      setAlertPolicyError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-alert-policies"] })
    },
    onError: (error) => setAlertPolicyError(error instanceof Error ? error.message : i18next.t("kisi.orgSetup.description")),
  })

  const saveIntegrationMutation = useMutation({
    mutationFn: () => {
      const nextTenantID = integrationDraft.tenant_id.trim() || defaultIntegrationTenantID
      if (!nextTenantID) {
        throw new Error("tenant_id is required")
      }
      if (!integrationDraft.provider.trim()) {
        throw new Error("provider is required")
      }
      if (integrationDraft.type === "identity_provider" && (!integrationDraft.issuer_url.trim() || !integrationDraft.client_id.trim()) && !editingIntegrationID) {
        throw new Error("issuer_url and client_id are required")
      }
      const payload = {
        tenant_id: nextTenantID,
        type: integrationDraft.type,
        provider: integrationDraft.provider.trim(),
        status: integrationDraft.status.trim() || "active",
        sync_mode: integrationDraft.sync_mode.trim(),
        credential_ref: integrationDraft.credential_ref.trim() || undefined,
        webhook_secret_ref: integrationDraft.webhook_secret_ref.trim() || undefined,
        issuer_url: integrationDraft.issuer_url.trim() || undefined,
        client_id: integrationDraft.client_id.trim() || undefined,
        auth_url: integrationDraft.auth_url.trim() || undefined,
        actor: viewer.email,
      }
      return editingIntegrationID
        ? updateIntegration(token, editingIntegrationID, payload)
        : createIntegration(token, payload)
    },
    onSuccess: async () => {
      setIntegrationSheetOpen(false)
      setEditingIntegrationID("")
      setIntegrationDraft(defaultIntegrationDraft(defaultIntegrationTenantID))
      setIntegrationError("")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["reference-integrations"] }),
        queryClient.invalidateQueries({ queryKey: ["reference-integration-detail"] }),
      ])
    },
    onError: (error) => setIntegrationError(error instanceof Error ? error.message : "Integration save failed"),
  })

  const deleteIntegrationMutation = useMutation({
    mutationFn: (integration: Integration) => deleteIntegration(token, integration.id, integration.tenant_id),
    onSuccess: async () => {
      setDeleteIntegrationTarget(null)
      setIntegrationError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-integrations"] })
    },
    onError: (error) => setIntegrationError(error instanceof Error ? error.message : "Integration disable failed"),
  })

  const fallbackAlertRows: AlertPolicyRow[] =
    activeTab === "Notifications"
      ? [
          [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "Send high severity events to #security-ops", "success"],
          [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "Email assigned Place Admins for place-scoped alerts", "success"],
          ["Weekly digest", "Review", "Summarize non-urgent access trends every Monday", "warning"],
        ]
      : activeTab === "Escalation"
        ? [
            [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "Escalate after 2 minutes without acknowledgement", "success"],
            [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "Escalate to hardware owner after 10 minutes", "success"],
            ["Repeated access denied", "Review", "Escalate after 3 denied attempts in 10 minutes", "warning"],
          ]
        : [
            [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "High severity, notify security team immediately", "success"],
            [i18next.t("kisi.orgSetup.notifications"), i18next.t("common.enabled"), "Notify place admins after 5 minutes", "success"],
            ["Repeated access denied", "Review", "Create review item after threshold is met", "warning"],
          ]
  const liveAlertRows = liveAlertPolicyRows(alertPoliciesQuery.data ?? [], activeTab)
  const alertRows = liveAlertRows.length > 0 ? liveAlertRows : fallbackAlertRows
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
              [i18next.t("common.permissions"), i18next.t("common.active"), "Apply per-tenant API protection", "success"],
            ]
          : [
              ["SSO", "Connected", "SAML identity provider for admin sign-in", "success"],
              ["SCIM", "Pending", "Directory sync token has been generated but not confirmed", "warning"],
              ["Webhook", "Connected", "Door, credential, and alarm event delivery", "success"],
              ["MQTT", "Available", "Gateway event stream for device integrations", "info"],
            ]
  const visibleIntegrations = visibleIntegrationsForTab(integrationsQuery.data ?? [], activeTab, isSsoScim)
  const liveRows = liveIntegrationRows(integrationsQuery.data ?? [], activeTab, isSsoScim)
  const integrationRows = liveRows.length > 0 ? liveRows : fallbackIntegrationRows
  const billingRows = [
    ["Current plan", "Active", "Enterprise Access · 3 places · 42 doors"],
    ["Next invoice", "Scheduled", "May 1, 2026"],
    ["Payment method", "Ready", "Visa ending in 4242"],
  ]

  return (
    <PageFrame
      breadcrumbs={[t("common.home"), "Organization Setup", title]}
      title={title}
      description={t("kisi.orgSetup.description")}
    >
      <SettingsPanel
        tabs={tabs}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            type="button"
            disabled={
              isAlertPolicies
                ? !alertPoliciesDirty || liveAlertPolicies.length === 0 || saveAlertPoliciesMutation.isPending
                : false
            }
            onClick={() => {
              if (isAlertPolicies) {
                saveAlertPoliciesMutation.mutate()
              }
            }}
            className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {isCreatePlace
              ? "Create Place"
              : isAlertPolicies
                ? saveAlertPoliciesMutation.isPending
                  ? t("kisi.orgSetup.saving")
                  : t("kisi.orgSetup.savePolicies")
                : t("kisi.orgSetup.saveChanges")}
          </Button>
        }
      >
        {isCreatePlace ? (
          <>
            <div className="border-b border-line-subtle px-7 py-5">
              <h2 className="text-lg font-semibold text-content-heading">{activeTab}</h2>
              <p className="mt-1 text-sm text-content-subtle">{t("kisi.orgSetup.createPlaceDesc")}</p>
            </div>
            <div className="mx-7 mt-4 rounded-[6px] border border-[#c9ccff] bg-brand-subtle px-5 py-4 text-sm text-[#3439cc]">
              Use the Places page to create places. This wizard view is a preview placeholder.
            </div>
            {activeTab === "General" ? (
              <div className="grid gap-6 p-7 md:grid-cols-2">
                <FormField label={t("common.name")} value="Sudirman Hub" />
                <FormField label={t("kisi.doors.timezone")} value="Asia/Jakarta" trailing={<ChevronDownIcon className="size-4 text-content-subtle" />} />
                <FormField label={t("common.description")} value="Jakarta, Indonesia" />
                <FormField label={t("common.placeAdmin")} value="Place Admin" />
              </div>
            ) : null}
            {activeTab === "Floors" ? (
              <div className="divide-y divide-line-subtle">
                {[
                  ["1st Floor", "Lobby and reception", "Default"],
                  ["3rd Floor", "Office and meeting rooms", "Ready"],
                  ["Parking", "Garage and loading bay", "Ready"],
                ].map((row, index) => (
                  <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_1fr_140px] md:items-center">
                    <span className="font-semibold text-content-heading">{row[0]}</span>
                    <span className="text-sm text-content-subtle">{row[1]}</span>
                    <StatusDot tone={index === 0 ? "info" : "success"} label={row[2]} />
                  </div>
                ))}
              </div>
            ) : null}
            {activeTab === "Doors" ? (
              <div className="divide-y divide-line-subtle">
                {[
                  ["Lobby Turnstile", "1st Floor", "Reader required"],
                  ["Garage", "Parking", "Mobile unlock enabled"],
                  ["11th Floor Entry", "3rd Floor", "Business hours"],
                ].map((row) => (
                  <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_160px_1fr] md:items-center">
                    <span className="font-semibold text-content-heading">{row[0]}</span>
                    <span className="text-sm text-content-body">{row[1]}</span>
                    <span className="text-sm text-content-subtle">{row[2]}</span>
                  </div>
                ))}
              </div>
            ) : null}
            {activeTab === "Hardware" ? (
              <SettingToggleRows
                rows={[
                  ["Register gateway after creation", true, "Create an onboarding slot for the first gateway.", ServerIcon],
                  ["Enable reader inventory", true, "Track controller and reader serials from day one.", CreditCardIcon],
                  [i18next.t("common.permissions"), false, i18next.t("kisi.orgSetup.description"), CloudIcon],
                ]}
              />
            ) : null}
          </>
        ) : null}

        {isAlertPolicies ? (
          <>
            <div className="flex items-center justify-between gap-4 border-b border-line-subtle px-7 py-5">
              <div>
                <h2 className="text-lg font-semibold text-content-heading">{activeTab}</h2>
                <p className="mt-1 text-sm text-content-subtle">{t("kisi.orgSetup.alertDesc")}</p>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={!defaultAlertPolicyTenantID || createAlertPolicyMutation.isPending}
                onClick={() => {
                  const nextCategory: AlertPolicyCategory = liveAlertPolicies.some((policy) => policy.category === "enterprise_sync_worker" && policy.enabled)
                    ? "wallet_jobs"
                    : "enterprise_sync_worker"
                  setCreateAlertPolicyCategory(nextCategory)
                  setCreateAlertPolicyDraft(defaultAlertPolicyDraft(nextCategory))
                  setCreateAlertPolicyTenantID(defaultAlertPolicyTenantID)
                  setAlertPolicyConditionPreview(null)
                  setAlertPolicyError("")
                  setCreateAlertPolicyOpen(true)
                }}
                className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-brand-subtle hover:text-[#3439cc] disabled:border-line-default disabled:text-content-muted"
              >
                <PlusIcon className="mr-1.5 size-4" />
                Add Policy
              </Button>
            </div>
            {alertPolicyError ? (
              <div className="border-b border-[#f1c27a] bg-warning-bg px-7 py-4 text-sm text-warning-text">
                {alertPolicyError}
              </div>
            ) : null}
            <div className="divide-y divide-line-subtle">
              {liveAlertPolicies.length > 0
                ? liveAlertPolicies.map((policy) => {
                    const draft = alertPolicyDrafts[policy.id] ?? alertPolicyDraftFromPolicy(policy)
                    const channels = [
                      draft.email ? "Email" : "",
                      draft.whatsapp ? "WhatsApp" : "",
                    ].filter(Boolean)
                    return (
                      <div key={policy.id} className="grid gap-5 px-7 py-5 xl:grid-cols-[40px_minmax(180px,1fr)_minmax(260px,1.2fr)_200px] xl:items-start">
                        <div className="flex size-10 shrink-0 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                          <AlertCircleIcon className="size-5 text-content-body" />
                        </div>
                        <div className="min-w-0">
                          <h3 className="font-semibold text-content-heading">{policy.name}</h3>
                          <p className="mt-1 text-sm leading-6 text-content-subtle">
                            {activeTab === "Notifications"
                              ? `${channels.length > 0 ? channels.join(" + ") : "No channels"} to ${splitReceiverGroups(draft.receiver_groups).join(", ") || "security"}`
                              : activeTab === "Escalation"
                                ? `Trigger ${policy.trigger}; cooldown ${formatPolicySeconds(Number.parseInt(draft.cooldown_seconds, 10) || 0)}`
                                : policy.category === "custom" && draft.condition_expression.trim()
                                  ? `${policy.description} Condition ${draft.condition_expression.trim()}.`
                                  : `${policy.description} Window ${formatPolicySeconds(Number.parseInt(draft.window_seconds, 10) || 0)}.`}
                          </p>
                        </div>
                        <div className="grid gap-3 sm:grid-cols-3">
                          {activeTab === "Notifications" ? (
                            <>
                              <button
                                type="button"
                                disabled={saveAlertPoliciesMutation.isPending}
                                onClick={() => updateAlertPolicyDraft(policy.id, { email: !draft.email })}
                                className="flex h-11 items-center justify-between rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-page"
                              >
                                Email
                                <ToggleSwitch enabled={draft.email} />
                              </button>
                              <button
                                type="button"
                                disabled={saveAlertPoliciesMutation.isPending}
                                onClick={() => updateAlertPolicyDraft(policy.id, { whatsapp: !draft.whatsapp })}
                                className="flex h-11 items-center justify-between rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-page"
                              >
                                WhatsApp
                                <ToggleSwitch enabled={draft.whatsapp} />
                              </button>
                              <label className="block">
                                <span className="sr-only">{t("kisi.orgSetup.receiverGroups")}</span>
                                <input
                                  value={draft.receiver_groups}
                                  disabled={saveAlertPoliciesMutation.isPending}
                                  onChange={(event) => updateAlertPolicyDraft(policy.id, { receiver_groups: event.target.value })}
                                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-page"
                                />
                              </label>
                            </>
                          ) : (
                            <>
                              <label className="block">
                                <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.threshold")}</span>
                                <input
                                  value={draft.threshold}
                                  disabled={saveAlertPoliciesMutation.isPending}
                                  inputMode="numeric"
                                  onChange={(event) => updateAlertPolicyDraft(policy.id, { threshold: event.target.value })}
                                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-page"
                                />
                              </label>
                              <label className="block">
                                <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.windowSeconds")}</span>
                                <input
                                  value={draft.window_seconds}
                                  disabled={saveAlertPoliciesMutation.isPending}
                                  inputMode="numeric"
                                  onChange={(event) => updateAlertPolicyDraft(policy.id, { window_seconds: event.target.value })}
                                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-page"
                                />
                              </label>
                              <label className="block">
                                <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.cooldownSeconds")}</span>
                                <input
                                  value={draft.cooldown_seconds}
                                  disabled={saveAlertPoliciesMutation.isPending}
                                  inputMode="numeric"
                                  onChange={(event) => updateAlertPolicyDraft(policy.id, { cooldown_seconds: event.target.value })}
                                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-page"
                                />
                              </label>
                            </>
                          )}
                        </div>
                        <div className="flex flex-wrap items-center justify-between gap-3 lg:justify-end">
                          <StatusDot tone={alertPolicyDraftTone(draft)} label={alertPolicyDraftStatusLabel(draft)} />
                          <button
                            type="button"
                            disabled={saveAlertPoliciesMutation.isPending || deleteAlertPolicyMutation.isPending}
                            onClick={() => updateAlertPolicyDraft(policy.id, { enabled: !draft.enabled })}
                            className="rounded-full disabled:opacity-60"
                            aria-pressed={draft.enabled}
                          >
                            <ToggleSwitch enabled={draft.enabled} />
                          </button>
                          <RowActionsMenu
                            label={`Actions for ${policy.name}`}
                            items={[
                              {
                                id: "delete",
                                label: "Delete",
                                icon: Trash2Icon,
                                destructive: true,
                                disabled: saveAlertPoliciesMutation.isPending || deleteAlertPolicyMutation.isPending,
                                onSelect: () => {
                                  setAlertPolicyError("")
                                  setDeleteAlertPolicyTarget(policy)
                                },
                              },
                            ]}
                          />
                        </div>
                      </div>
                    )
                  })
                : alertRows.map((row) => (
                    <div key={row[0]} className="flex gap-5 px-7 py-5">
                      <div className="flex size-10 shrink-0 items-center justify-center rounded-[6px] bg-[#f1f2f5]">
                        <AlertCircleIcon className="size-5 text-content-body" />
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-semibold text-content-heading">{row[0]}</h3>
                        <p className="mt-1 text-sm text-content-subtle">{row[2]}</p>
                      </div>
                      <div className="ml-auto flex items-center gap-4">
                        <StatusDot tone={row[3]} label={row[1]} />
                        <ToggleSwitch enabled={row[3] === "success"} />
                      </div>
                    </div>
                  ))}
            </div>
          </>
        ) : null}

        {isIntegrations || isSsoScim ? (
          <>
            <div className="flex items-center justify-between gap-4 border-b border-line-subtle px-7 py-5">
              <div>
                <h2 className="text-lg font-semibold text-content-heading">{activeTab}</h2>
                <p className="mt-1 text-sm text-content-subtle">{t("kisi.orgSetup.integrationsDesc")}</p>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={!defaultIntegrationTenantID || saveIntegrationMutation.isPending}
                onClick={openCreateIntegrationSheet}
                className="h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-brand-subtle hover:text-[#3439cc] disabled:border-line-default disabled:text-content-muted"
              >
                <PlusIcon className="mr-1.5 size-4" />
                Add Integration
              </Button>
            </div>
            {integrationError ? (
              <div className="border-b border-[#f1c27a] bg-warning-bg px-7 py-4 text-sm text-warning-text">
                {integrationError}
              </div>
            ) : null}
            <div className="grid gap-4 p-7 md:grid-cols-2">
              {visibleIntegrations.length > 0
                ? visibleIntegrations.map((integration) => (
                    <div key={integration.id} className="rounded-[6px] border border-line-subtle p-5">
                      <div className="flex items-start gap-4">
                        <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-[6px]", integrationStatus(integration) === "warning" ? "bg-warning-bg" : "bg-[#f1f2f5]")}>
                          <ShieldCheckIcon className="size-5 text-content-body" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-start gap-3">
                            <div className="min-w-0 flex-1">
                              <h3 className="font-semibold text-content-heading">{integration.name}</h3>
                              <p className="mt-1 text-sm leading-6 text-content-subtle">{integration.description}</p>
                            </div>
                            <RowActionsMenu
                              label={`Actions for ${integration.name}`}
                              items={[
                                {
                                  id: "edit",
                                  label: "Edit",
                                  icon: ShieldCheckIcon,
                                  onSelect: () => openEditIntegrationSheet(integration),
                                },
                                {
                                  id: "disable",
                                  label: "Disable",
                                  icon: Trash2Icon,
                                  destructive: true,
                                  disabled: deleteIntegrationMutation.isPending || integration.status === "inactive",
                                  onSelect: () => {
                                    setIntegrationError("")
                                    setDeleteIntegrationTarget(integration)
                                  },
                                },
                              ]}
                            />
                          </div>
                          <div className="mt-4 grid gap-3 text-sm text-content-subtle sm:grid-cols-2">
                            <FormField label={t("kisi.orgSetup.provider")} value={integration.provider.toUpperCase()} />
                            <FormField label={t("kisi.orgSetup.syncMode")} value={integration.sync_mode || "Manual"} />
                            <FormField label={t("common.name")} value={integration.source_id || integration.id} />
                            <FormField label={t("common.status")} value={integration.last_sync_at || "Not synced"} />
                          </div>
                          <div className="mt-4">
                            <StatusDot tone={integrationStatus(integration)} label={integrationStatusLabel(integration)} />
                          </div>
                        </div>
                      </div>
                    </div>
                  ))
                : (isSsoScim ? integrationRows.slice(0, 2) : integrationRows).map((row, index) => (
                    <div key={row[0]} className="rounded-[6px] border border-line-subtle p-5">
                      <div className="flex items-start gap-4">
                        <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-[6px]", index === 1 ? "bg-warning-bg" : "bg-[#f1f2f5]")}>
                          <ShieldCheckIcon className="size-5 text-content-body" />
                        </div>
                        <div className="min-w-0">
                          <h3 className="font-semibold text-content-heading">{row[0]}</h3>
                          <p className="mt-1 text-sm leading-6 text-content-subtle">{row[2]}</p>
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
            <div className="border-b border-line-subtle px-7 py-5">
              <h2 className="text-lg font-semibold text-content-heading">{activeTab}</h2>
              <p className="mt-1 text-sm text-content-subtle">{t("kisi.orgSetup.billingDesc")}</p>
            </div>
            <div className="mx-7 mt-4 rounded-[6px] border border-[#c9ccff] bg-brand-subtle px-5 py-4 text-sm text-[#3439cc]">
              Billing management is coming soon. The settings below are preview placeholders.
            </div>
            <div className="divide-y divide-line-subtle">
              {billingRows.map((row, index) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-content-heading">{row[0]}</span>
                  <StatusDot tone={index === 1 ? "info" : "success"} label={row[1]} />
                  <span className="text-sm text-content-subtle">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {isSettings ? (
          <OrganizationSettingsSection token={token} tenantID={tenantID} activeTab={activeTab} />
        ) : null}
      </SettingsPanel>

      <ConfirmActionDialog
        open={Boolean(deleteIntegrationTarget)}
        onOpenChange={(open) => {
          if (!deleteIntegrationMutation.isPending && !open) {
            setDeleteIntegrationTarget(null)
          }
        }}
        title={t("common.disabled")}
        description={
          <>
            This marks <span className="font-semibold text-content-heading">{deleteIntegrationTarget?.name ?? "this integration"}</span>{" "}
            as inactive while keeping its audit history.
          </>
        }
        confirmLabel="Disable integration"
        pending={deleteIntegrationMutation.isPending}
        disabled={!deleteIntegrationTarget}
        destructive
        onConfirm={() => {
          if (deleteIntegrationTarget) {
            deleteIntegrationMutation.mutate(deleteIntegrationTarget)
          }
        }}
      />

      <Sheet
        open={integrationSheetOpen}
        onOpenChange={(open) => {
          if (!saveIntegrationMutation.isPending) {
            setIntegrationSheetOpen(open)
            if (!open) {
              setEditingIntegrationID("")
              setIntegrationError("")
            }
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[520px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{editingIntegrationID ? "Edit Integration" : "Add Integration"}</SheetTitle>
            <SheetDescription>
              {editingIntegrationID ? "Review detail and update integration status or sync mode." : "Create an identity provider or HRIS integration."}
            </SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              saveIntegrationMutation.mutate()
            }}
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.tenantId")}</span>
                <input
                  value={integrationDraft.tenant_id}
                  disabled={Boolean(tenantID) || saveIntegrationMutation.isPending}
                  onChange={(event) => updateIntegrationDraft({ tenant_id: event.target.value })}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-sunken"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.type")}</span>
                <select
                  value={integrationDraft.type}
                  disabled={Boolean(editingIntegrationID) || saveIntegrationMutation.isPending}
                  onChange={(event) => {
                    const nextType = event.target.value as IntegrationDraft["type"]
                    setIntegrationDraft({
                      ...defaultIntegrationDraft(integrationDraft.tenant_id || defaultIntegrationTenantID),
                      type: nextType,
                      provider: nextType === "identity_provider" ? "oidc" : "talenta",
                      sync_mode: nextType === "identity_provider" ? "jit" : "hybrid",
                    })
                  }}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-sunken"
                >
                  <option value="hris">{t("kisi.orgSetup.hris")}</option>
                  <option value="identity_provider">{t("kisi.orgSetup.identityProvider")}</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.provider")}</span>
                <select
                  value={integrationDraft.provider}
                  disabled={Boolean(editingIntegrationID) || saveIntegrationMutation.isPending}
                  onChange={(event) => updateIntegrationDraft({ provider: event.target.value })}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-sunken"
                >
                  {integrationDraft.type === "identity_provider" ? (
                    <>
                      <option value="oidc">OIDC</option>
                      <option value="saml">SAML</option>
                    </>
                  ) : (
                    <>
                      <option value="talenta">Talenta</option>
                      <option value="gadjian">Gadjian</option>
                      <option value="greatday">GreatDay</option>
                      <option value="linovhr">LinovHR</option>
                      <option value="sunfish">Sunfish</option>
                    </>
                  )}
                </select>
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.status")}</span>
                <select
                  value={integrationDraft.status}
                  disabled={saveIntegrationMutation.isPending}
                  onChange={(event) => updateIntegrationDraft({ status: event.target.value })}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-sunken"
                >
                  <option value="active">{t("common.active")}</option>
                  <option value="inactive">{t("common.inactive")}</option>
                </select>
              </label>
              <label className="block sm:col-span-2">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.syncMode")}</span>
                <select
                  value={integrationDraft.sync_mode}
                  disabled={saveIntegrationMutation.isPending}
                  onChange={(event) => updateIntegrationDraft({ sync_mode: event.target.value })}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body disabled:bg-surface-sunken"
                >
                  {integrationDraft.type === "identity_provider" ? (
                    <>
                      <option value="jit">JIT</option>
                      <option value="manual">{t("kisi.orgSetup.manual")}</option>
                      <option value="scheduled">{t("common.scheduled")}</option>
                    </>
                  ) : (
                    <>
                      <option value="hybrid">{t("kisi.orgSetup.hybrid")}</option>
                      <option value="webhook">{t("kisi.orgSetup.webhook")}</option>
                      <option value="pull">{t("kisi.orgSetup.pull")}</option>
                    </>
                  )}
                </select>
              </label>
            </div>

            {integrationDraft.type === "identity_provider" ? (
              <div className="grid gap-4">
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.issuerUrl")}</span>
                  <input
                    value={integrationDraft.issuer_url}
                    disabled={saveIntegrationMutation.isPending}
                    onChange={(event) => updateIntegrationDraft({ issuer_url: event.target.value })}
                    placeholder={editingIntegrationID ? "Leave blank to keep current issuer" : "https://idp.example.com"}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.clientId")}</span>
                  <input
                    value={integrationDraft.client_id}
                    disabled={saveIntegrationMutation.isPending}
                    onChange={(event) => updateIntegrationDraft({ client_id: event.target.value })}
                    placeholder={editingIntegrationID ? "Leave blank to keep current client" : "misty-admin"}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.authUrl")}</span>
                  <input
                    value={integrationDraft.auth_url}
                    disabled={saveIntegrationMutation.isPending}
                    onChange={(event) => updateIntegrationDraft({ auth_url: event.target.value })}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
              </div>
            ) : (
              <div className="grid gap-4">
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.credentialRef")}</span>
                  <input
                    value={integrationDraft.credential_ref}
                    disabled={saveIntegrationMutation.isPending}
                    onChange={(event) => updateIntegrationDraft({ credential_ref: event.target.value })}
                    placeholder="vault://tenant/hris/provider/api_key"
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.webhookSecretRef")}</span>
                  <input
                    value={integrationDraft.webhook_secret_ref}
                    disabled={saveIntegrationMutation.isPending}
                    onChange={(event) => updateIntegrationDraft({ webhook_secret_ref: event.target.value })}
                    placeholder="vault://tenant/hris/provider/webhook_secret"
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
              </div>
            )}

            {integrationDetailQuery.isPending && editingIntegrationID ? (
              <div className="rounded-[6px] border border-line-subtle bg-surface-page px-4 py-3 text-sm text-content-subtle">
                Loading integration detail...
              </div>
            ) : null}
            {integrationError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-warning-bg px-4 py-3 text-sm text-warning-text">
                {integrationError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setIntegrationSheetOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={saveIntegrationMutation.isPending || (Boolean(editingIntegrationID) && integrationDetailQuery.isPending)}
                className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {saveIntegrationMutation.isPending ? t("kisi.orgSetup.saving") : editingIntegrationID ? "Save Integration" : "Add Integration"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <ConfirmActionDialog
        open={Boolean(deleteAlertPolicyTarget)}
        onOpenChange={(open) => {
          if (!deleteAlertPolicyMutation.isPending && !open) {
            setDeleteAlertPolicyTarget(null)
          }
        }}
        title={t("common.delete")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{deleteAlertPolicyTarget?.name ?? "this policy"}</span>{" "}
            from alert policy monitoring.
          </>
        }
        confirmLabel="Delete policy"
        pending={deleteAlertPolicyMutation.isPending}
        disabled={!deleteAlertPolicyTarget}
        destructive
        onConfirm={() => {
          if (deleteAlertPolicyTarget) {
            deleteAlertPolicyMutation.mutate(deleteAlertPolicyTarget.id)
          }
        }}
      />

      <Sheet open={createAlertPolicyOpen} onOpenChange={setCreateAlertPolicyOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{t("kisi.orgSetup.addPolicy")}</SheetTitle>
            <SheetDescription>{t("kisi.orgSetup.addPolicyDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createAlertPolicyMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.tenantId")}</span>
              <input
                value={createAlertPolicyTenantID}
                readOnly={Boolean(tenantID)}
                onChange={(event) => {
                  setCreateAlertPolicyTenantID(event.target.value)
                  setAlertPolicyConditionPreview(null)
                }}
                className={cn(
                  "h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body",
                  tenantID ? "bg-surface-page" : "bg-white"
                )}
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.policyType")}</span>
              <select
                value={createAlertPolicyCategory}
                onChange={(event) => {
                  const category = event.target.value as AlertPolicyCategory
                  setCreateAlertPolicyCategory(category)
                  setCreateAlertPolicyDraft(defaultAlertPolicyDraft(category))
                  setAlertPolicyConditionPreview(null)
                }}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                <option value="enterprise_sync_worker">{t("kisi.orgSetup.enterpriseSync")}</option>
                <option value="wallet_jobs">{t("kisi.orgSetup.walletJobs")}</option>
                <option value="custom">{t("kisi.orgSetup.customCondition")}</option>
              </select>
            </label>
            {createAlertPolicyCategory === "custom" ? (
              <>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.policyName")}</span>
                  <input
                    value={createAlertPolicyDraft.name}
                    onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, name: event.target.value }))}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.trigger")}</span>
                  <input
                    value={createAlertPolicyDraft.trigger}
                    onChange={(event) => {
                      setCreateAlertPolicyDraft((draft) => ({ ...draft, trigger: event.target.value }))
                      setAlertPolicyConditionPreview(null)
                    }}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.severity")}</span>
                  <select
                    value={createAlertPolicyDraft.severity}
                    onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, severity: event.target.value }))}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  >
                    <option value="info">{t("kisi.orgSetup.info")}</option>
                    <option value="warning">{t("kisi.orgSetup.warning")}</option>
                    <option value="high">{t("kisi.orgSetup.high")}</option>
                    <option value="critical">{t("kisi.orgSetup.critical")}</option>
                  </select>
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.conditionExpr")}</span>
                  <input
                    value={createAlertPolicyDraft.condition_expression}
                    onChange={(event) => {
                      setCreateAlertPolicyDraft((draft) => ({ ...draft, condition_expression: event.target.value }))
                      setAlertPolicyConditionPreview(null)
                    }}
                    className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                  />
                </label>
                <div className="flex items-center justify-between gap-3 rounded-[6px] border border-line-default bg-white px-3 py-3">
                  <span className="text-sm font-semibold text-content-heading">
                    {alertPolicyConditionPreview ? (alertPolicyConditionPreview.matched ? "Matched" : "Not matched") : "No preview"}
                  </span>
                  <Button
                    type="button"
                    disabled={
                      previewAlertPolicyConditionMutation.isPending ||
                      !createAlertPolicyDraft.condition_expression.trim() ||
                      !(createAlertPolicyTenantID.trim() || defaultAlertPolicyTenantID)
                    }
                    onClick={() => previewAlertPolicyConditionMutation.mutate()}
                    className="h-9 rounded-[6px] border border-line-default bg-white px-3 text-content-body hover:bg-[#f6f7fb]"
                  >
                    <BarChart3Icon className="mr-2 h-4 w-4" />
                    {previewAlertPolicyConditionMutation.isPending ? "Previewing..." : "Preview"}
                  </Button>
                </div>
              </>
            ) : null}
            <button
              type="button"
              disabled={createAlertPolicyMutation.isPending}
              onClick={() => setCreateAlertPolicyDraft((draft) => ({ ...draft, enabled: !draft.enabled }))}
              className="flex w-full items-center justify-between rounded-[6px] border border-line-default bg-white px-3 py-3 text-left disabled:bg-surface-page"
            >
              <span>
                <span className="block text-sm font-semibold text-content-heading">{t("kisi.orgSetup.policyEnabled")}</span>
                <span className="mt-1 block text-xs text-content-subtle">{t("kisi.orgSetup.policyEnabledDesc")}</span>
              </span>
              <ToggleSwitch enabled={createAlertPolicyDraft.enabled} />
            </button>
            <div className="grid gap-3 sm:grid-cols-3">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.threshold")}</span>
                <input
                  value={createAlertPolicyDraft.threshold}
                  inputMode="numeric"
                  onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, threshold: event.target.value }))}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.window")}</span>
                <input
                  value={createAlertPolicyDraft.window_seconds}
                  inputMode="numeric"
                  onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, window_seconds: event.target.value }))}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.cooldown")}</span>
                <input
                  value={createAlertPolicyDraft.cooldown_seconds}
                  inputMode="numeric"
                  onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, cooldown_seconds: event.target.value }))}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                />
              </label>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <button
                type="button"
                disabled={createAlertPolicyMutation.isPending}
                onClick={() => setCreateAlertPolicyDraft((draft) => ({ ...draft, email: !draft.email }))}
                className="flex h-11 items-center justify-between rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-page"
              >
                Email
                <ToggleSwitch enabled={createAlertPolicyDraft.email} />
              </button>
              <button
                type="button"
                disabled={createAlertPolicyMutation.isPending}
                onClick={() => setCreateAlertPolicyDraft((draft) => ({ ...draft, whatsapp: !draft.whatsapp }))}
                className="flex h-11 items-center justify-between rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-page"
              >
                WhatsApp
                <ToggleSwitch enabled={createAlertPolicyDraft.whatsapp} />
              </button>
            </div>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.orgSetup.receiverGroups")}</span>
              <input
                value={createAlertPolicyDraft.receiver_groups}
                onChange={(event) => setCreateAlertPolicyDraft((draft) => ({ ...draft, receiver_groups: event.target.value }))}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={
                  createAlertPolicyMutation.isPending ||
                  !(createAlertPolicyTenantID.trim() || defaultAlertPolicyTenantID) ||
                  (createAlertPolicyCategory === "custom" && !createAlertPolicyDraft.trigger.trim()) ||
                  !createAlertPolicyDraft.threshold.trim() ||
                  !createAlertPolicyDraft.window_seconds.trim() ||
                  !createAlertPolicyDraft.cooldown_seconds.trim()
                }
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-[#454bea]"
              >
                {createAlertPolicyMutation.isPending ? "Creating..." : "Create Policy"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </PageFrame>
  )
}

function OrganizationSettingsSection({
  token,
  tenantID,
  activeTab,
}: {
  token: string
  tenantID: string | undefined
  activeTab: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [settingsNotice, setSettingsNotice] = useState("")
  const [confirmRotate, setConfirmRotate] = useState(false)
  const [confirmDisable, setConfirmDisable] = useState(false)

  const settingsQuery = useQuery({
    queryKey: ["orgSettings", tenantID],
    queryFn: () => getOrganizationSettings(token, tenantID),
    enabled: !!token,
  })

  const updateMutation = useMutation({
    mutationFn: (patch: Partial<OrganizationSettings>) =>
      updateOrganizationSettings(token, { ...patch, tenant_id: tenantID }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["orgSettings"] })
      setSettingsNotice(t("kisi.orgSetup.saveChanges") + " " + t("common.saved"))
    },
  })

  const exportMutation = useMutation({
    mutationFn: () => exportOrganizationAudit(token, tenantID),
    onSuccess: (res) => setSettingsNotice(res.message),
  })
  const rotateMutation = useMutation({
    mutationFn: () => rotateOrganizationWebhooks(token, tenantID),
    onSuccess: (res) => { setSettingsNotice(res.message); setConfirmRotate(false) },
  })
  const disableMutation = useMutation({
    mutationFn: () => disableOrganization(token, tenantID),
    onSuccess: (res) => { setSettingsNotice(res.message); setConfirmDisable(false) },
  })

  const s = settingsQuery.data

  function handleToggle(field: keyof OrganizationSettings, value: boolean) {
    updateMutation.mutate({ [field]: value } as Partial<OrganizationSettings>)
  }

  return (
    <>
      <div className="border-b border-line-subtle px-7 py-5">
        <h2 className="text-lg font-semibold text-content-heading">{activeTab}</h2>
        <p className="mt-1 text-sm text-content-subtle">{t("kisi.orgSetup.settingsDesc")}</p>
      </div>
      {settingsNotice && (
        <div className="mx-7 mt-4 rounded-[6px] border border-[#b7e4c7] bg-[#f0faf4] px-5 py-3 text-sm text-[#1a7f37]">
          {settingsNotice}
        </div>
      )}
      <div className="space-y-6 p-7">
        {activeTab === "Communication" ? (
          <SettingToggleRows
            rows={[
              [i18next.t("common.email"), s?.email_notifications ?? true, i18next.t("kisi.orgSetup.description"), UsersIcon],
              [i18next.t("kisi.orgSetup.notifications"), s?.push_notifications ?? true, i18next.t("kisi.orgSetup.description"), BellIcon],
              [i18next.t("kisi.reports.title"), s?.weekly_reports ?? false, i18next.t("kisi.orgSetup.description"), BarChart3Icon],
            ]}
            onToggle={(index) => {
              const fields: Array<keyof OrganizationSettings> = ["email_notifications", "push_notifications", "weekly_reports"]
              const current = [s?.email_notifications ?? true, s?.push_notifications ?? true, s?.weekly_reports ?? false]
              handleToggle(fields[index], !current[index])
            }}
          />
        ) : activeTab === "Security" ? (
          <div className="space-y-5">
            <div className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
              <div className="flex size-10 items-center justify-center rounded-[6px] bg-brand-subtle">
                <ShieldCheckIcon className="size-5 text-[#4f55ff]" />
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-content-heading">{t("kisi.myAccount.mfaTitle")}</h3>
                <p className="mt-1 text-sm text-content-subtle">{t("kisi.myAccount.mfaDescription")}</p>
              </div>
              <div className="pt-1">
                <ToggleSwitch enabled={s?.enforce_mfa ?? false} onToggle={() => handleToggle("enforce_mfa", !(s?.enforce_mfa ?? false))} />
              </div>
            </div>
            <div className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
              <div className="flex size-10 items-center justify-center rounded-[6px] bg-brand-subtle">
                <FingerprintIcon className="size-5 text-[#4f55ff]" />
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-content-heading">WebAuthn Sign-in</h3>
                <p className="mt-1 text-sm text-content-subtle">Allow users to sign in with passkeys (Touch ID, Face ID, security keys). Requires SSO enabled.</p>
              </div>
              <div className="pt-1">
                <ToggleSwitch enabled={s?.webauthn_enabled ?? false} onToggle={() => handleToggle("webauthn_enabled", !(s?.webauthn_enabled ?? false))} />
              </div>
            </div>
            <div className="grid gap-6 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">Password policy</span>
                <select
                  value={s?.password_policy ?? "standard"}
                  onChange={(e) => updateMutation.mutate({ password_policy: e.target.value })}
                  className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
                >
                  <option value="standard">Standard</option>
                  <option value="strict">Strict</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">Session timeout (minutes)</span>
                <input
                  type="number"
                  value={s?.session_timeout_minutes ?? 480}
                  onChange={(e) => updateMutation.mutate({ session_timeout_minutes: parseInt(e.target.value, 10) || 480 })}
                  min={5}
                  max={1440}
                  className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
                />
              </label>
            </div>
          </div>
        ) : activeTab === "Advanced" ? (
          <div className="space-y-4">
            <div className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
              <div>
                <h3 className="font-semibold text-content-heading">Export organization audit log</h3>
                <p className="mt-1 text-sm text-content-subtle">Generate an organization-wide event archive.</p>
              </div>
              <Button
                variant="outline"
                className="ml-auto h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-brand-subtle hover:text-[#3439cc]"
                disabled={exportMutation.isPending}
                onClick={() => exportMutation.mutate()}
              >
                {exportMutation.isPending ? "Exporting..." : "Export"}
              </Button>
            </div>
            <div className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
              <div>
                <h3 className="font-semibold text-content-heading">Rotate webhook secrets</h3>
                <p className="mt-1 text-sm text-content-subtle">Rotate all organization webhook signing secrets.</p>
              </div>
              <Button
                variant="outline"
                className="ml-auto h-10 rounded-[6px] border-[#8589ff] bg-white px-5 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-brand-subtle hover:text-[#3439cc]"
                onClick={() => setConfirmRotate(true)}
              >
                Rotate
              </Button>
            </div>
            <div className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
              <div>
                <h3 className="font-semibold text-content-heading">Disable organization</h3>
                <p className="mt-1 text-sm text-content-subtle">Reserved destructive action for tenant lifecycle operations.</p>
              </div>
              <Button
                variant="outline"
                className="ml-auto h-10 rounded-[6px] border-[#f1b7b2] bg-white px-5 text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff5f5] hover:text-[#9f1d1d]"
                onClick={() => setConfirmDisable(true)}
              >
                Disable
              </Button>
            </div>
          </div>
        ) : (
          <OrgSettingsGeneralForm settings={s} onSave={(patch) => updateMutation.mutate(patch)} saving={updateMutation.isPending} />
        )}
      </div>
      <ConfirmActionDialog
        open={confirmRotate}
        title="Rotate webhook secrets"
        description="This will invalidate all existing webhook secrets. Continue?"
        confirmLabel="Rotate"
        onConfirm={() => rotateMutation.mutate()}
        onOpenChange={(open) => { if (!open) setConfirmRotate(false) }}
        pending={rotateMutation.isPending}
      />
      <ConfirmActionDialog
        open={confirmDisable}
        onOpenChange={(open) => { if (!open) setConfirmDisable(false) }}
        title="Disable organization"
        description="This will disable the organization and suspend all access. This action cannot be easily reversed."
        confirmLabel="Disable"
        onConfirm={() => disableMutation.mutate()}
        pending={disableMutation.isPending}
        destructive
      />
    </>
  )
}

function OrgSettingsGeneralForm({
  settings,
  onSave,
  saving,
}: {
  settings: OrganizationSettings | undefined
  onSave: (patch: Partial<OrganizationSettings>) => void
  saving: boolean
}) {
  const [name, setName] = useState("")
  const [domain, setDomain] = useState("")
  const [timezone, setTimezone] = useState("")
  const [supportEmail, setSupportEmail] = useState("")
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    if (settings) {
      setName(settings.name)
      setDomain(settings.primary_domain)
      setTimezone(settings.timezone)
      setSupportEmail(settings.support_email)
      setDirty(false)
    }
  }, [settings])

  function handleSave() {
    onSave({ name, primary_domain: domain, timezone, support_email: supportEmail })
    setDirty(false)
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-6 md:grid-cols-2">
        <label className="block">
          <span className="mb-2 block text-xs font-semibold text-content-subtle">Organization name</span>
          <input
            value={name}
            onChange={(e) => { setName(e.target.value); setDirty(true) }}
            className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none transition focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
          />
        </label>
        <label className="block">
          <span className="mb-2 block text-xs font-semibold text-content-subtle">Primary domain</span>
          <input
            value={domain}
            onChange={(e) => { setDomain(e.target.value); setDirty(true) }}
            className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none transition focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
          />
        </label>
        <label className="block">
          <span className="mb-2 block text-xs font-semibold text-content-subtle">Timezone</span>
          <input
            value={timezone}
            onChange={(e) => { setTimezone(e.target.value); setDirty(true) }}
            className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none transition focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
          />
        </label>
        <label className="block">
          <span className="mb-2 block text-xs font-semibold text-content-subtle">Support email</span>
          <input
            value={supportEmail}
            onChange={(e) => { setSupportEmail(e.target.value); setDirty(true) }}
            className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body outline-none transition focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
          />
        </label>
      </div>
      {dirty && (
        <Button
          className="h-10 rounded-[6px] px-6"
          disabled={saving}
          onClick={handleSave}
        >
          {saving ? "Saving..." : "Save"}
        </Button>
      )}
    </div>
  )
}
