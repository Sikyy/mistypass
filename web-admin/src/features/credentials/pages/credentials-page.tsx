import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ClipboardListIcon,
  DownloadIcon,
  EyeIcon,
  PlusIcon,
  RotateCcwIcon,
  RotateCwIcon,
  SendIcon,
  SnowflakeIcon,
  Trash2Icon,
} from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow, MistyisletFilterButton, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import {
  activateCard,
  assignCard,
  batchUpdateWalletPhysicalCardInventoryStatus,
  createCard,
  createWalletPhysicalCardTask,
  deactivateCard,
  deassignCard,
  dispatchWalletPassDelivery,
  getCard,
  getCardAssignment,
  importWalletPhysicalCardInventoryCSV,
  issueWalletPassBatch,
  listWalletPhysicalCardInventory,
  listWalletJobs,
  listWalletPassDeliveries,
  listWalletPhysicalCardTasks,
  listWalletPhysicalCardVendors,
  retryWalletPassDelivery,
  revokeCard,
  scanWalletPhysicalCardInventoryItem,
  type Card,
  type CardAssignment,
  type CurrentUser,
  type WalletIssueJob,
  type WalletPassDeliveryNotification,
  type WalletPhysicalCardInventoryGovernanceStatus,
  type WalletPhysicalCardInventoryItem,
  type WalletPhysicalCardTask,
  type WalletPhysicalCardVendor,
  updateWalletPhysicalCardInventoryStatus,
  updateWalletPhysicalCardTaskStatus,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

import {
  credentialBatchIssueJobsToCSV,
  filterCredentialBatchIssueJobs,
  formatCredentialBatchIssueNotice,
  summarizeCredentialBatchIssue,
  type CredentialBatchJobStatusFilter,
  walletIssueJobErrorLabel,
  walletIssueJobStatusTone,
  walletIssueJobTargetLabel,
} from "./credentials-page-utils"

function credentialStatusLabel(status: Card["status"]) {
  if (status === "activated") {
    return "Active"
  }
  if (status === "unassigned") {
    return "Pending"
  }
  if (status === "revoked") {
    return "Revoked"
  }
  return "Suspended"
}

function credentialTone(status: Card["status"]) {
  if (status === "activated") {
    return "success" as const
  }
  if (status === "unassigned") {
    return "warning" as const
  }
  return "danger" as const
}

function statusText(value: string) {
  return value
    .split("_")
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ")
}

function credentialKindLabel(kind: Card["credential_kind"]) {
  if (kind === "google_wallet") {
    return "Google Wallet"
  }
  if (kind === "apple_wallet") {
    return "Apple Wallet"
  }
  if (kind === "physical_card") {
    return "Physical Card"
  }
  return "Credential"
}

function detailDateLabel(value?: string) {
  if (!value?.trim()) {
    return "None"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function CredentialDetailField({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <dt className="text-xs font-semibold uppercase text-[#6f717c]">{label}</dt>
      <dd className="mt-1 break-all text-sm font-medium text-[#2f3037]">{value?.trim() || "None"}</dd>
    </div>
  )
}

function nextPhysicalTaskStatuses(task: WalletPhysicalCardTask): WalletPhysicalCardTask["status"][] {
  if (task.task_type === "loss_report") {
    if (task.status === "queued") {
      return ["reported_lost", "cancelled"]
    }
    return []
  }
  if (task.status === "queued") {
    return ["printing", "ready", "issued", "cancelled"]
  }
  if (task.status === "printing") {
    return ["ready", "issued", "cancelled"]
  }
  if (task.status === "ready") {
    return ["issued", "cancelled"]
  }
  return []
}

const batchStatusFilters: Array<{ value: CredentialBatchJobStatusFilter; label: string }> = [
  { value: "all", label: "All statuses" },
  { value: "success", label: "Issued" },
  { value: "queued", label: "Queued" },
  { value: "failed", label: "Failed" },
]

type CredentialAction = "activate" | "deactivate" | "deassign" | "revoke"
type CredentialActionTarget = {
  id: string
  action: CredentialAction
  label: string
}
type PhysicalTaskStatusTarget = {
  task: WalletPhysicalCardTask
  status: WalletPhysicalCardTask["status"]
}
type PhysicalInventoryStatusTarget = {
  item: WalletPhysicalCardInventoryItem
  status: WalletPhysicalCardInventoryGovernanceStatus
}
type PhysicalInventoryBatchStatusTarget = {
  inventoryIDs: string[]
  status: WalletPhysicalCardInventoryGovernanceStatus
}

function canTransitionPhysicalInventoryStatus(
  current: WalletPhysicalCardInventoryItem["status"],
  next: WalletPhysicalCardInventoryGovernanceStatus
) {
  if (current === next) {
    return false
  }
  if (current === "available") {
    return next === "frozen" || next === "scrapped"
  }
  if (current === "frozen") {
    return next === "available" || next === "scrapped"
  }
  if (current === "lost") {
    return next === "scrapped"
  }
  return false
}

function physicalInventoryStatusTone(status: WalletPhysicalCardInventoryItem["status"]) {
  if (status === "available") {
    return "success" as const
  }
  if (status === "frozen" || status === "reserved") {
    return "warning" as const
  }
  if (status === "issued") {
    return "info" as const
  }
  return "danger" as const
}

function physicalInventoryStatusDialogTitle(status?: WalletPhysicalCardInventoryGovernanceStatus) {
  if (status === "available") {
    return "Return inventory card"
  }
  if (status === "frozen") {
    return "Freeze inventory card"
  }
  if (status === "scrapped") {
    return "Scrap inventory card"
  }
  return "Update inventory card"
}

function physicalInventoryStatusDescription(status?: WalletPhysicalCardInventoryGovernanceStatus) {
  if (status === "available") {
    return "This returns the inventory card to the available pool."
  }
  if (status === "frozen") {
    return "This holds the inventory card so it cannot be used for new physical card tasks."
  }
  if (status === "scrapped") {
    return "This permanently removes the inventory card from usable stock."
  }
  return "This updates the inventory card status."
}

function credentialActionTitle(action?: CredentialAction) {
  if (action === "deactivate") {
    return "Deactivate credential"
  }
  if (action === "deassign") {
    return "Deassign credential"
  }
  if (action === "revoke") {
    return "Revoke credential"
  }
  return "Update credential"
}

function credentialActionConfirmLabel(action?: CredentialAction) {
  if (action === "deactivate") {
    return "Deactivate"
  }
  if (action === "deassign") {
    return "Deassign"
  }
  if (action === "revoke") {
    return "Revoke"
  }
  return "Update"
}

function credentialActionDescription(action?: CredentialAction) {
  if (action === "deactivate") {
    return "This suspends access until the credential is activated again."
  }
  if (action === "deassign") {
    return "This removes the user assignment from the credential."
  }
  if (action === "revoke") {
    return "This permanently revokes the credential."
  }
  return "This updates the credential."
}

function physicalTaskStatusNeedsConfirmation(status: WalletPhysicalCardTask["status"]) {
  return status === "issued" || status === "reported_lost" || status === "cancelled"
}

function physicalTaskStatusDialogTitle(status?: WalletPhysicalCardTask["status"]) {
  if (status === "issued") {
    return "Mark card issued"
  }
  if (status === "reported_lost") {
    return "Report card lost"
  }
  if (status === "cancelled") {
    return "Cancel physical card task"
  }
  return "Update physical card task"
}

function physicalTaskStatusDescription(status?: WalletPhysicalCardTask["status"]) {
  if (status === "issued") {
    return "This marks the physical card as issued and completes the task."
  }
  if (status === "reported_lost") {
    return "This reports the physical card as lost and updates inventory state."
  }
  if (status === "cancelled") {
    return "This cancels the physical card task and releases any available inventory."
  }
  return "This updates the physical card task."
}

export function CredentialsAdaptedPage({
  token,
  viewer,
}: {
  token: string
  viewer: CurrentUser
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const rows = resourceQuery.summary.credentials
  const users = resourceQuery.summary.users
  const tenantID = getViewerTenantID(viewer)
  const [activeTab, setActiveTab] = useState("Active")
  const [query, setQuery] = useState("")
  const [issueOpen, setIssueOpen] = useState(false)
  const [issueMode, setIssueMode] = useState<"single" | "batch">("single")
  const [selectedUserID, setSelectedUserID] = useState("")
  const [selectedBatchUserIDs, setSelectedBatchUserIDs] = useState<string[]>([])
  const [batchQuery, setBatchQuery] = useState("")
  const [batchStatusFilter, setBatchStatusFilter] = useState<CredentialBatchJobStatusFilter>("all")
  const [uid, setUID] = useState("")
  const [expiresAt, setExpiresAt] = useState("")
  const [detailOpen, setDetailOpen] = useState(false)
  const [selectedCardID, setSelectedCardID] = useState("")
  const [detailAssigneeType, setDetailAssigneeType] = useState<"User" | "Guest">("User")
  const [detailAssigneeID, setDetailAssigneeID] = useState("")
  const [detailGuestID, setDetailGuestID] = useState("")
  const [deliveryEmailRecipients, setDeliveryEmailRecipients] = useState("")
  const [deliveryWhatsAppRecipients, setDeliveryWhatsAppRecipients] = useState("")
  const [physicalTaskType, setPhysicalTaskType] = useState<WalletPhysicalCardTask["task_type"]>("issue")
  const [physicalCardNumber, setPhysicalCardNumber] = useState("")
  const [physicalInventoryID, setPhysicalInventoryID] = useState("")
  const [physicalVendorID, setPhysicalVendorID] = useState("")
  const [physicalTaskNote, setPhysicalTaskNote] = useState("")
  const [physicalScanUID, setPhysicalScanUID] = useState("")
  const [physicalScanCardNumber, setPhysicalScanCardNumber] = useState("")
  const [physicalScanReaderID, setPhysicalScanReaderID] = useState("")
  const [physicalInventoryCSV, setPhysicalInventoryCSV] = useState("")
  const [selectedPhysicalInventoryIDs, setSelectedPhysicalInventoryIDs] = useState<string[]>([])
  const [physicalInventoryBatchStatus, setPhysicalInventoryBatchStatus] =
    useState<WalletPhysicalCardInventoryGovernanceStatus>("frozen")
  const [credentialActionTarget, setCredentialActionTarget] = useState<CredentialActionTarget | null>(null)
  const [physicalTaskStatusTarget, setPhysicalTaskStatusTarget] = useState<PhysicalTaskStatusTarget | null>(null)
  const [physicalInventoryStatusTarget, setPhysicalInventoryStatusTarget] = useState<PhysicalInventoryStatusTarget | null>(null)
  const [physicalInventoryBatchStatusTarget, setPhysicalInventoryBatchStatusTarget] =
    useState<PhysicalInventoryBatchStatusTarget | null>(null)
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const visibleRows = rows.filter((row) => {
    const matchesTab = row.statusLabel === activeTab
    const matchesQuery =
      query.trim() === "" ||
      [row.user, row.type, row.statusLabel, row.issuedLabel, row.expiresLabel].join(" ").toLowerCase().includes(query.trim().toLowerCase())
    return matchesTab && matchesQuery
  })
  const tabs = ["Active", "Pending", "Suspended", "Revoked"]
  const selectedUser = users.find((user) => user.id === selectedUserID) ?? users[0]
  const selectedBatchUsers = users.filter((user) => selectedBatchUserIDs.includes(user.id))
  const detailCardQuery = useQuery<Card>({
    queryKey: ["credential-card-detail", selectedCardID, tenantID],
    queryFn: () => {
      if (!selectedCardID) {
        throw new Error("credential is required")
      }
      return getCard(token, selectedCardID, tenantID)
    },
    enabled: detailOpen && Boolean(selectedCardID),
  })
  const walletJobsQuery = useQuery<WalletIssueJob[]>({
    queryKey: ["credential-wallet-jobs", tenantID],
    queryFn: () => listWalletJobs(token, tenantID),
    enabled: Boolean(tenantID),
  })
  const detailAssignmentQuery = useQuery<CardAssignment>({
    queryKey: ["credential-card-assignment-detail", selectedCardID, tenantID],
    queryFn: () => {
      if (!selectedCardID) {
        throw new Error("credential is required")
      }
      return getCardAssignment(token, `ca_${selectedCardID}`, tenantID)
    },
    enabled: detailOpen && Boolean(selectedCardID),
  })
  const physicalTasksQuery = useQuery<WalletPhysicalCardTask[]>({
    queryKey: ["credential-physical-card-tasks", tenantID],
    queryFn: () => listWalletPhysicalCardTasks(token, tenantID),
    enabled: detailOpen && Boolean(tenantID),
  })
  const physicalVendorsQuery = useQuery<WalletPhysicalCardVendor[]>({
    queryKey: ["credential-physical-card-vendors", tenantID],
    queryFn: () => listWalletPhysicalCardVendors(token, tenantID),
    enabled: detailOpen && Boolean(tenantID),
  })
  const physicalInventoryQuery = useQuery<WalletPhysicalCardInventoryItem[]>({
    queryKey: ["credential-physical-card-inventory", tenantID],
    queryFn: () => listWalletPhysicalCardInventory(token, { tenant_id: tenantID }),
    enabled: detailOpen && Boolean(tenantID),
  })
  const deliveriesQuery = useQuery<WalletPassDeliveryNotification[]>({
    queryKey: ["credential-deliveries", selectedCardID, tenantID],
    queryFn: () => {
      if (!tenantID || !selectedCardID) {
        throw new Error("credential is required")
      }
      return listWalletPassDeliveries(token, { tenant_id: tenantID, pass_id: selectedCardID })
    },
    enabled: detailOpen && Boolean(tenantID && selectedCardID),
  })
  const detailCard = detailCardQuery.data
  const detailAssignment = detailAssignmentQuery.data
  const detailPhysicalTasks = (physicalTasksQuery.data ?? []).filter((task) => task.pass_id === selectedCardID)
  const detailDeliveries = deliveriesQuery.data ?? []
  const physicalVendors = physicalVendorsQuery.data ?? []
  const physicalInventory = physicalInventoryQuery.data ?? []
  const availablePhysicalInventory = physicalInventory.filter((item) => item.status === "available")
  const listedPhysicalInventory = physicalInventory.slice(0, 8)
  const selectedPhysicalInventoryIDSet = new Set(selectedPhysicalInventoryIDs)
  const selectedPhysicalInventoryCount = selectedPhysicalInventoryIDs.length
  const physicalInventoryCounts = physicalInventory.reduce(
    (counts, item) => {
      counts.total += 1
      counts[item.status] = (counts[item.status] ?? 0) + 1
      return counts
    },
    {
      total: 0,
      available: 0,
      reserved: 0,
      issued: 0,
      lost: 0,
      frozen: 0,
      scrapped: 0,
    } as Record<WalletPhysicalCardInventoryItem["status"] | "total", number>
  )
  const selectedPhysicalInventory = physicalInventoryID
    ? physicalInventory.find((item) => item.id === physicalInventoryID)
    : availablePhysicalInventory.find((item) => item.card_number === physicalCardNumber)
  const batchJobs = (walletJobsQuery.data ?? []).filter((job) => job.batch_id)
  const filteredBatchJobs = filterCredentialBatchIssueJobs(batchJobs, users, {
    query: batchQuery,
    status: batchStatusFilter,
  }).slice(0, 12)
  const batchAuditSummary = summarizeCredentialBatchIssue(filteredBatchJobs)
  const canSubmitAssignment =
    Boolean(tenantID && detailCard?.id) &&
    (detailAssigneeType === "User" ? Boolean(detailAssigneeID.trim()) : Boolean(detailGuestID.trim()))
  const canCreatePhysicalTask = Boolean(tenantID && detailCard?.id && detailCard.assignee_type === "User")
  const deliveryChannels = [
    deliveryEmailRecipients.trim() ? "email" : "",
    deliveryWhatsAppRecipients.trim() ? "whatsapp" : "",
  ].filter(Boolean)
  const canDispatchDelivery = Boolean(tenantID && detailCard?.id && deliveryChannels.length > 0)

  useEffect(() => {
    if (!detailCard) {
      return
    }
    if (detailCard.assignee_type === "Guest") {
      setDetailAssigneeType("Guest")
      setDetailAssigneeID("")
      setDetailGuestID(detailCard.assignee_id ?? "")
      return
    }
    setDetailAssigneeType("User")
    setDetailAssigneeID(detailCard.user_id || detailCard.assignee_id || users[0]?.id || "")
    setDetailGuestID("")
  }, [detailCard, users])

  useEffect(() => {
    if (!detailCard) {
      return
    }
    const user = users.find((item) => item.id === (detailCard.user_id || detailCard.assignee_id))
    setDeliveryEmailRecipients(user?.email ?? "")
    setDeliveryWhatsAppRecipients("")
    setPhysicalTaskType(detailCard.status === "revoked" ? "loss_report" : "issue")
    setPhysicalCardNumber(detailCard.card_number ?? "")
    setPhysicalInventoryID("")
    setPhysicalVendorID("")
    setPhysicalTaskNote("")
    setPhysicalScanUID("")
    setPhysicalScanCardNumber("")
    setPhysicalScanReaderID("")
  }, [detailCard, users])

  useEffect(() => {
    if (!selectedPhysicalInventory) {
      return
    }
    setPhysicalCardNumber(selectedPhysicalInventory.card_number)
    setPhysicalVendorID(selectedPhysicalInventory.vendor_id ?? "")
  }, [selectedPhysicalInventory])

  useEffect(() => {
    if (selectedPhysicalInventoryIDs.length === 0) {
      return
    }
    const inventoryIDs = new Set(physicalInventory.map((item) => item.id))
    setSelectedPhysicalInventoryIDs((current) => current.filter((id) => inventoryIDs.has(id)))
  }, [physicalInventory, selectedPhysicalInventoryIDs.length])

  async function refreshCredentials() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-card-detail"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-card-assignment-detail"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-wallet-jobs"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-physical-card-tasks"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-physical-card-vendors"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-physical-card-inventory"] }),
      queryClient.invalidateQueries({ queryKey: ["credential-deliveries"] }),
    ])
  }

  function openDetail(cardID: string) {
    setSelectedCardID(cardID)
    setActionNotice("")
    setActionError("")
    setDetailOpen(true)
  }

  function requestCredentialAction(target: CredentialActionTarget) {
    setActionError("")
    if (target.action === "activate") {
      credentialActionMutation.mutate({ id: target.id, action: target.action })
      return
    }
    setCredentialActionTarget(target)
  }

  function requestPhysicalTaskStatus(target: PhysicalTaskStatusTarget) {
    setActionError("")
    if (physicalTaskStatusNeedsConfirmation(target.status)) {
      setPhysicalTaskStatusTarget(target)
      return
    }
    physicalTaskStatusMutation.mutate(target)
  }

  function requestPhysicalInventoryStatus(target: PhysicalInventoryStatusTarget) {
    setActionError("")
    setPhysicalInventoryStatusTarget(target)
  }

  function requestPhysicalInventoryBatchStatus() {
    setActionError("")
    if (selectedPhysicalInventoryIDs.length === 0) {
      setActionError("Select at least one inventory card.")
      return
    }
    const selectedItems = physicalInventory.filter((item) => selectedPhysicalInventoryIDs.includes(item.id))
    const invalidItem = selectedItems.find((item) => !canTransitionPhysicalInventoryStatus(item.status, physicalInventoryBatchStatus))
    if (invalidItem) {
      setActionError(`${invalidItem.card_number} cannot move from ${statusText(invalidItem.status)} to ${statusText(physicalInventoryBatchStatus)}.`)
      return
    }
    setPhysicalInventoryBatchStatusTarget({
      inventoryIDs: selectedPhysicalInventoryIDs,
      status: physicalInventoryBatchStatus,
    })
  }

  function togglePhysicalInventorySelection(inventoryID: string) {
    setSelectedPhysicalInventoryIDs((current) =>
      current.includes(inventoryID) ? current.filter((item) => item !== inventoryID) : [...current, inventoryID]
    )
  }

  function toggleBatchUser(userID: string) {
    setSelectedBatchUserIDs((current) =>
      current.includes(userID) ? current.filter((item) => item !== userID) : [...current, userID]
    )
  }

  function exportBatchAuditCSV() {
    if (filteredBatchJobs.length === 0) {
      return
    }
    const csv = credentialBatchIssueJobsToCSV(filteredBatchJobs, users)
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement("a")
    link.href = url
    link.download = `credential-batch-audit-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    setActionNotice(`Exported ${filteredBatchJobs.length} batch audit row${filteredBatchJobs.length === 1 ? "" : "s"}.`)
    setActionError("")
  }

  const issueMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (issueMode === "batch") {
        if (selectedBatchUserIDs.length === 0) {
          throw new Error("select at least one user")
        }
        return issueWalletPassBatch(token, {
          tenant_id: tenantID,
          template_id: "wpt_employee_demo",
          target_type: "user",
          target_ids: selectedBatchUserIDs,
          expires_at: expiresAt.trim() || undefined,
          execution_mode: "inline",
        })
      }
      const userID = selectedUser?.id
      if (!userID) {
        throw new Error("user is required")
      }
      return createCard(token, {
        tenant_id: tenantID,
        template_id: "wpt_employee_demo",
        uid: uid.trim() || undefined,
        type: uid.trim() ? "third_party_hf" : undefined,
        assignee_type: "User",
        user_id: userID,
        expires_at: expiresAt.trim() || undefined,
      })
    },
    onSuccess: async (result) => {
      setIssueOpen(false)
      setUID("")
      setExpiresAt("")
      setSelectedBatchUserIDs([])
      if ("items" in result) {
        setActionNotice(formatCredentialBatchIssueNotice(result.items))
      } else {
        setActionNotice("Credential issued.")
      }
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Credential issue failed"),
  })

  const credentialActionMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: CredentialAction }) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (action === "activate") {
        return activateCard(token, id, tenantID)
      }
      if (action === "deassign") {
        return deassignCard(token, id, tenantID)
      }
      if (action === "revoke") {
        return revokeCard(token, id, tenantID)
      }
      return deactivateCard(token, id, tenantID)
    },
    onSuccess: async () => {
      setCredentialActionTarget(null)
      setActionNotice("Credential updated.")
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Credential action failed"),
  })

  const detailAssignmentMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!detailCard?.id) {
        throw new Error("credential is required")
      }
      if (detailAssigneeType === "Guest") {
        const guestID = detailGuestID.trim()
        if (!guestID) {
          throw new Error("guest is required")
        }
        return assignCard(token, detailCard.id, {
          tenant_id: tenantID,
          assignee_type: "Guest",
          guest_id: guestID,
          email: guestID,
        })
      }
      const userID = detailAssigneeID.trim()
      if (!userID) {
        throw new Error("user is required")
      }
      return assignCard(token, detailCard.id, {
        tenant_id: tenantID,
        assignee_type: "User",
        assignee_id: userID,
        user_id: userID,
      })
    },
    onSuccess: async () => {
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Credential assignment failed"),
  })

  const deliveryMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !detailCard?.id) {
        throw new Error("credential is required")
      }
      const channels = deliveryChannels as Array<"email" | "whatsapp">
      if (channels.length === 0) {
        throw new Error("delivery channel is required")
      }
      return dispatchWalletPassDelivery(token, {
        tenant_id: tenantID,
        pass_id: detailCard.id,
        channels,
        email_recipients: deliveryEmailRecipients
          .split(/[,\n]/)
          .map((item) => item.trim())
          .filter(Boolean),
        whatsapp_recipients: deliveryWhatsAppRecipients
          .split(/[,\n]/)
          .map((item) => item.trim())
          .filter(Boolean),
      })
    },
    onSuccess: async () => {
      setActionNotice("Delivery dispatched.")
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Credential delivery failed"),
  })

  const retryDeliveryMutation = useMutation({
    mutationFn: (notificationID: string) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return retryWalletPassDelivery(token, { tenant_id: tenantID, notification_id: notificationID })
    },
    onSuccess: async () => {
      setActionNotice("Delivery retry queued.")
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Delivery retry failed"),
  })

  const physicalTaskMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !detailCard?.id) {
        throw new Error("credential is required")
      }
      return createWalletPhysicalCardTask(token, {
        tenant_id: tenantID,
        pass_id: detailCard.id,
        task_type: physicalTaskType,
        card_number: physicalCardNumber.trim() || undefined,
        inventory_id: selectedPhysicalInventory?.id || physicalInventoryID.trim() || undefined,
        vendor_id: physicalVendorID.trim() || selectedPhysicalInventory?.vendor_id || undefined,
        note: physicalTaskNote.trim() || undefined,
      })
    },
    onSuccess: async () => {
      setPhysicalTaskNote("")
      setActionNotice("Physical card task created.")
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card task failed"),
  })

  const physicalInventoryImportMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!physicalInventoryCSV.trim()) {
        throw new Error("inventory csv is required")
      }
      return importWalletPhysicalCardInventoryCSV(token, {
        tenant_id: tenantID,
        csv_content: physicalInventoryCSV,
      })
    },
    onSuccess: async (items) => {
      setPhysicalInventoryCSV("")
      setActionNotice(`Imported ${items.length} physical card${items.length === 1 ? "" : "s"}.`)
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card inventory import failed"),
  })

  const physicalInventoryScanMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!physicalScanUID.trim()) {
        throw new Error("uid is required")
      }
      return scanWalletPhysicalCardInventoryItem(token, {
        tenant_id: tenantID,
        uid: physicalScanUID.trim(),
        card_number: physicalScanCardNumber.trim() || undefined,
        reader_id: physicalScanReaderID.trim() || undefined,
        vendor_id: physicalVendorID.trim() || selectedPhysicalInventory?.vendor_id || undefined,
      })
    },
    onSuccess: async (item) => {
      setPhysicalInventoryID(item.id)
      setPhysicalCardNumber(item.card_number)
      setPhysicalVendorID(item.vendor_id ?? "")
      setPhysicalScanUID("")
      setPhysicalScanCardNumber("")
      setPhysicalScanReaderID("")
      setActionNotice(`Scanned ${item.card_number}.`)
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card scan failed"),
  })

  const physicalInventoryStatusMutation = useMutation({
    mutationFn: ({ item, status }: PhysicalInventoryStatusTarget) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return updateWalletPhysicalCardInventoryStatus(token, item.id, {
        tenant_id: tenantID,
        status,
        reason: `web_admin:${status}`,
      })
    },
    onSuccess: async (item) => {
      setPhysicalInventoryStatusTarget(null)
      setSelectedPhysicalInventoryIDs((current) => current.filter((id) => id !== item.id))
      setActionNotice(`Inventory ${item.card_number} updated to ${statusText(item.status)}.`)
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card inventory update failed"),
  })

  const physicalInventoryBatchStatusMutation = useMutation({
    mutationFn: ({ inventoryIDs, status }: PhysicalInventoryBatchStatusTarget) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return batchUpdateWalletPhysicalCardInventoryStatus(token, {
        tenant_id: tenantID,
        inventory_ids: inventoryIDs,
        status,
        reason: `web_admin_batch:${status}`,
      })
    },
    onSuccess: async (items) => {
      setPhysicalInventoryBatchStatusTarget(null)
      setSelectedPhysicalInventoryIDs([])
      setActionNotice(`Updated ${items.length} inventory card${items.length === 1 ? "" : "s"}.`)
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card inventory batch update failed"),
  })

  const physicalTaskStatusMutation = useMutation({
    mutationFn: ({ task, status }: PhysicalTaskStatusTarget) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return updateWalletPhysicalCardTaskStatus(token, task.id, {
        tenant_id: tenantID,
        status,
        card_number: task.card_number,
        note: task.note,
      })
    },
    onSuccess: async () => {
      setPhysicalTaskStatusTarget(null)
      setActionNotice("Physical card task updated.")
      setActionError("")
      await refreshCredentials()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Physical card task update failed"),
  })

  return (
    <PageFrame
      breadcrumbs={[t("common.home"), "Credentials"]}
      title="Credentials"
      count={resourceQuery.isPending ? "--" : rows.length}
      description="Issue and monitor access credentials"
      actions={
        <Button
          type="button"
          onClick={() => {
            setIssueMode("single")
            setSelectedUserID(selectedUserID || users[0]?.id || "")
            setSelectedBatchUserIDs(users.slice(0, 3).map((user) => user.id))
            setActionNotice("")
            setActionError("")
            setIssueOpen(true)
          }}
          className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
        >
          <PlusIcon className="mr-1.5 size-4" />
          Issue Credential
        </Button>
      }
    >
      {resourceQuery.summary.partial ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Some live credential resources are unavailable.
        </div>
      ) : null}
      {actionNotice ? (
        <div className="rounded-[6px] border border-[#b9dfc7] bg-[#f1fff5] px-5 py-4 text-sm text-[#1f6b3a]">
          {actionNotice}
        </div>
      ) : null}
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          {actionError}
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex flex-col gap-3 border-b border-[#eceef2] px-6 py-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold text-[#17171c]">{t("kisi.credentials.batchAudit")}</h2>
            <p className="mt-1 text-sm text-[#6f717c]">
              {walletJobsQuery.isError
                ? "Batch jobs unavailable"
                : walletJobsQuery.isPending
                  ? "Loading jobs..."
                  : `${filteredBatchJobs.length} shown of ${batchJobs.length} jobs · ${batchAuditSummary.succeeded} issued · ${batchAuditSummary.queued} queued · ${batchAuditSummary.failed} failed`}
            </p>
          </div>
          <ClipboardListIcon className="size-5 text-[#6f717c]" />
        </div>
        <div className="flex flex-col gap-3 border-b border-[#eceef2] bg-[#fbfbfc] p-5 lg:flex-row lg:items-center">
          <MistyisletSearchField
            value={batchQuery}
            onChange={setBatchQuery}
            placeholder="Search batch, target, credential, error..."
            className="lg:max-w-[440px]"
          />
          <select
            value={batchStatusFilter}
            onChange={(event) => setBatchStatusFilter(event.target.value as CredentialBatchJobStatusFilter)}
            className="h-10 rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm font-semibold text-[#2f3037]"
          >
            {batchStatusFilters.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
          <Button
            type="button"
            variant="outline"
            disabled={filteredBatchJobs.length === 0 || walletJobsQuery.isPending}
            onClick={exportBatchAuditCSV}
            className="h-10 rounded-[6px] lg:ml-auto"
          >
            <DownloadIcon className="mr-1.5 size-4" />
            Export CSV
          </Button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[900px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">{t("kisi.credentials.batchAudit")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.target")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.credentials.title")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.description")}</th>
                <th className="px-6 py-4 font-semibold">{t("common.status")}</th>
              </tr>
            </thead>
            <tbody>
              {walletJobsQuery.isError ? (
                <MistyisletEmptyTableRow colSpan={6}>Batch jobs unavailable.</MistyisletEmptyTableRow>
              ) : null}
              {walletJobsQuery.isPending ? (
                <MistyisletEmptyTableRow colSpan={6}>{t("common.loading")}</MistyisletEmptyTableRow>
              ) : null}
              {!walletJobsQuery.isPending && !walletJobsQuery.isError && filteredBatchJobs.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={6}>
                  {batchJobs.length === 0 ? "No batch jobs yet." : "No batch jobs match this view."}
                </MistyisletEmptyTableRow>
              ) : null}
              {filteredBatchJobs.map((job) => (
                <tr key={job.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="max-w-[180px] break-all px-6 py-4 font-semibold text-[#17171c]">{job.batch_id}</td>
                  <td className="max-w-[260px] break-words px-4 py-4 text-[#2f3037]">
                    {walletIssueJobTargetLabel(job, users)}
                  </td>
                  <td className="px-4 py-4">
                    <StatusDot tone={walletIssueJobStatusTone(job.status)} label={statusText(job.status)} />
                  </td>
                  <td className="max-w-[180px] break-all px-4 py-4 text-[#2f3037]">
                    {job.pass_id ? (
                      <button
                        type="button"
                        onClick={() => openDetail(job.pass_id ?? "")}
                        className="font-semibold text-[#4f55ff] underline-offset-2 hover:underline"
                      >
                        {job.pass_id}
                      </button>
                    ) : (
                      "None"
                    )}
                  </td>
                  <td className="max-w-[220px] break-words px-4 py-4 text-[#6f717c]">{walletIssueJobErrorLabel(job)}</td>
                  <td className="px-6 py-4 text-[#6f717c]">{detailDateLabel(job.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="flex gap-10 border-b border-[#eceef2] px-6">
          {tabs.map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn("py-5 text-base font-semibold", activeTab === tab ? "border-b-2 border-[#4f55ff] text-[#4f55ff]" : "text-[#2f3037]")}
            >
              {tab}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-3 border-b border-[#eceef2] bg-[#fbfbfc] p-5">
          <MistyisletSearchField value={query} onChange={setQuery} placeholder="Search credentials..." />
          <MistyisletFilterButton label="Type" className="gap-2" />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[900px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-[#2f3037]">
                <th className="px-6 py-4 font-semibold">User</th>
                <th className="px-4 py-4 font-semibold">{t("common.type")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.credentials.issued")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.validUntil")}</th>
                <th className="px-6 py-4 text-right font-semibold">{t("common.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => (
                <tr key={row.id} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                  <td className="px-6 py-5 font-semibold text-[#17171c]">{row.user}</td>
                  <td className="px-4 py-5 text-[#2f3037]">{row.type}</td>
                  <td className="px-4 py-5">
                    <StatusDot tone={row.tone} label={row.statusLabel} />
                  </td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.issuedLabel}</td>
                  <td className="px-4 py-5 text-[#6f717c]">{row.expiresLabel}</td>
                  <td className="px-6 py-5 text-right">
                    <RowActionsMenu
                      label={`Actions for ${row.user}`}
                      items={[
                        {
                          id: "details",
                          label: "Details",
                          icon: EyeIcon,
                          onSelect: () => openDetail(row.id),
                        },
                        {
                          id: "activate",
                          label: "Activate",
                          hidden: row.statusLabel !== "Suspended",
                          disabled: credentialActionMutation.isPending,
                          onSelect: () => requestCredentialAction({ id: row.id, action: "activate", label: row.user }),
                        },
                        {
                          id: "deactivate",
                          label: "Deactivate",
                          hidden: row.statusLabel !== "Active",
                          disabled: credentialActionMutation.isPending,
                          destructive: true,
                          onSelect: () => requestCredentialAction({ id: row.id, action: "deactivate", label: row.user }),
                        },
                        {
                          id: "deassign",
                          label: "Deassign",
                          hidden: row.statusLabel !== "Active" && row.statusLabel !== "Suspended",
                          disabled: credentialActionMutation.isPending,
                          destructive: true,
                          onSelect: () => requestCredentialAction({ id: row.id, action: "deassign", label: row.user }),
                        },
                        {
                          id: "revoke",
                          label: "Revoke",
                          hidden: row.statusLabel === "Revoked",
                          disabled: credentialActionMutation.isPending,
                          destructive: true,
                          onSelect: () => requestCredentialAction({ id: row.id, action: "revoke", label: row.user }),
                        },
                      ]}
                    />
                  </td>
                </tr>
              ))}
              {visibleRows.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={6}>No {activeTab.toLowerCase()} credentials match this view.</MistyisletEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <Sheet open={issueOpen} onOpenChange={setIssueOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.credentials.issue")}</SheetTitle>
            <SheetDescription>Issue a Google Wallet credential or register a physical UID.</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              issueMutation.mutate()
            }}
          >
            <div className="grid grid-cols-2 gap-2 rounded-[6px] bg-[#f3f4f8] p-1">
              {(["single", "batch"] as const).map((mode) => (
                <button
                  key={mode}
                  type="button"
                  disabled={issueMutation.isPending}
                  onClick={() => {
                    setIssueMode(mode)
                    if (mode === "batch" && selectedBatchUserIDs.length === 0) {
                      setSelectedBatchUserIDs(users.slice(0, 3).map((user) => user.id))
                    }
                  }}
                  className={cn(
                    "h-9 rounded-[5px] text-sm font-semibold",
                    issueMode === mode ? "bg-white text-[#17171c] shadow-sm" : "text-[#6f717c]"
                  )}
                >
                  {mode === "single" ? "Single" : "Batch"}
                </button>
              ))}
            </div>
            {issueMode === "single" ? (
              <>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">User</span>
                  <select
                    value={selectedUser?.id ?? ""}
                    onChange={(event) => setSelectedUserID(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  >
                    {users.map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.name} · {user.email}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">UID</span>
                  <input
                    value={uid}
                    onChange={(event) => setUID(event.target.value)}
                    placeholder="Optional physical card UID"
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                  />
                </label>
              </>
            ) : (
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <span className="block text-xs font-semibold text-[#6f717c]">Users</span>
                  <button
                    type="button"
                    onClick={() => setSelectedBatchUserIDs(selectedBatchUserIDs.length === users.length ? [] : users.map((user) => user.id))}
                    className="text-xs font-semibold text-[#4f55ff]"
                  >
                    {selectedBatchUserIDs.length === users.length ? "Clear" : "Select all"}
                  </button>
                </div>
                <div className="max-h-56 overflow-y-auto rounded-[6px] border border-[#d9dbe3]">
                  {users.map((user) => (
                    <label key={user.id} className="flex items-center gap-3 border-b border-[#eceef2] px-3 py-3 last:border-0">
                      <input
                        type="checkbox"
                        checked={selectedBatchUserIDs.includes(user.id)}
                        onChange={() => toggleBatchUser(user.id)}
                        className="size-4 rounded border-[#9a9ca7]"
                      />
                      <span className="min-w-0">
                        <span className="block truncate text-sm font-semibold text-[#2f3037]">{user.name}</span>
                        <span className="block truncate text-xs text-[#6f717c]">{user.email}</span>
                      </span>
                    </label>
                  ))}
                </div>
                <p className="mt-2 text-xs text-[#6f717c]">{selectedBatchUsers.length} selected</p>
              </div>
            )}
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.validUntil")}</span>
              <input
                type="datetime-local"
                value={expiresAt}
                onChange={(event) => setExpiresAt(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            {users.length === 0 ? <p className="text-sm text-[#8a5a00]">No users are available for assignment.</p> : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={issueMutation.isPending || users.length === 0 || (issueMode === "batch" && selectedBatchUserIDs.length === 0)}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {issueMutation.isPending ? "Issuing..." : issueMode === "batch" ? "Issue Batch" : "Issue Credential"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet
        open={detailOpen}
        onOpenChange={(open) => {
          setDetailOpen(open)
          if (!open) {
            setSelectedCardID("")
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[540px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.credentials.title")}</SheetTitle>
            <SheetDescription>{selectedCardID || "Selected credential"}</SheetDescription>
          </SheetHeader>
          {detailCardQuery.isPending ? (
            <div className="px-6 py-5 text-sm text-[#6f717c]">{t("common.loading")}</div>
          ) : detailCardQuery.isError ? (
            <div className="mx-6 mt-5 rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
              {detailCardQuery.error instanceof Error ? detailCardQuery.error.message : "Credential detail unavailable"}
            </div>
          ) : detailCard ? (
            <div className="space-y-6 px-6 py-5">
              <section className="rounded-[6px] border border-[#d9dbe3] p-4">
                <div className="flex items-center justify-between gap-3 border-b border-[#eceef2] pb-4">
                  <div>
                    <p className="text-sm font-semibold text-[#17171c]">{detailCard.card_number || detailCard.id}</p>
                    <p className="mt-1 text-xs text-[#6f717c]">{detailCard.template_id}</p>
                  </div>
                  <StatusDot tone={credentialTone(detailCard.status)} label={credentialStatusLabel(detailCard.status)} />
                </div>
                <dl className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <CredentialDetailField label="Credential ID" value={detailCard.id} />
                  <CredentialDetailField label="Credential type" value={credentialKindLabel(detailCard.credential_kind)} />
                  <CredentialDetailField label="Token" value={detailCard.token} />
                  <CredentialDetailField label="UID" value={detailCard.uid} />
                  <CredentialDetailField label="Card number" value={detailCard.card_number} />
                  <CredentialDetailField label="Provider" value={detailCard.provider} />
                  <CredentialDetailField label="Save link" value={detailCard.save_link} />
                  <CredentialDetailField label="Activation token" value={detailCard.activation_token} />
                  <CredentialDetailField label="Issued" value={detailDateLabel(detailCard.issued_at)} />
                  <CredentialDetailField label="Expires" value={detailDateLabel(detailCard.expires_at)} />
                  <CredentialDetailField label="Last used" value={detailDateLabel(detailCard.last_used_at)} />
                  <CredentialDetailField label="Updated" value={detailDateLabel(detailCard.updated_at)} />
                </dl>
              </section>

              <section className="rounded-[6px] border border-[#d9dbe3] p-4">
                <div className="border-b border-[#eceef2] pb-4">
                  <h3 className="text-sm font-semibold text-[#17171c]">{t("kisi.credentials.assign")}</h3>
                  <p className="mt-1 text-xs text-[#6f717c]">{detailAssignmentQuery.isPending ? "Loading assignment..." : detailAssignment?.id || "No assignment"}</p>
                </div>
                <dl className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <CredentialDetailField label="Assignment status" value={detailAssignment?.status ? credentialStatusLabel(detailAssignment.card.status) : "None"} />
                  <CredentialDetailField label="Assignee type" value={detailCard.assignee_type} />
                  <CredentialDetailField label="Assignee ID" value={detailCard.assignee_id || detailCard.user_id} />
                  <CredentialDetailField label="User ID" value={detailCard.user_id || detailAssignment?.user_id} />
                </dl>
                <form
                  className="mt-5 space-y-4"
                  onSubmit={(event) => {
                    event.preventDefault()
                    detailAssignmentMutation.mutate()
                  }}
                >
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee type</span>
                      <select
                        value={detailAssigneeType}
                        onChange={(event) => {
                          const nextType = event.target.value as "User" | "Guest"
                          setDetailAssigneeType(nextType)
                          if (nextType === "User") {
                            setDetailAssigneeID(users[0]?.id ?? "")
                            setDetailGuestID("")
                          } else {
                            setDetailAssigneeID("")
                          }
                        }}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="User">User</option>
                        <option value="Guest">Guest</option>
                      </select>
                    </label>
                    {detailAssigneeType === "User" ? (
                      <label className="block">
                        <span className="mb-2 block text-xs font-semibold text-[#6f717c]">User</span>
                        <select
                          value={detailAssigneeID}
                          onChange={(event) => setDetailAssigneeID(event.target.value)}
                          className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                        >
                          {users.map((user) => (
                            <option key={user.id} value={user.id}>
                              {user.name} · {user.email}
                            </option>
                          ))}
                        </select>
                      </label>
                    ) : (
                      <label className="block">
                        <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Guest</span>
                        <input
                          value={detailGuestID}
                          onChange={(event) => setDetailGuestID(event.target.value)}
                          placeholder="Guest email or ID"
                          className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                        />
                      </label>
                    )}
                  </div>
                  <Button
                    type="submit"
                    disabled={!canSubmitAssignment || detailAssignmentMutation.isPending}
                    className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
                  >
                    {detailAssignmentMutation.isPending ? "Saving..." : "Save Assignment"}
                  </Button>
                </form>
              </section>

              <section className="rounded-[6px] border border-[#d9dbe3] p-4">
                <div className="flex items-center justify-between gap-3 border-b border-[#eceef2] pb-4">
                  <div>
                    <h3 className="text-sm font-semibold text-[#17171c]">Delivery</h3>
                    <p className="mt-1 text-xs text-[#6f717c]">{deliveriesQuery.isPending ? "Loading deliveries..." : `${detailDeliveries.length} attempts`}</p>
                  </div>
                  <SendIcon className="size-4 text-[#6f717c]" />
                </div>
                <form
                  className="mt-4 space-y-3"
                  onSubmit={(event) => {
                    event.preventDefault()
                    deliveryMutation.mutate()
                  }}
                >
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Email recipients</span>
                    <input
                      value={deliveryEmailRecipients}
                      onChange={(event) => setDeliveryEmailRecipients(event.target.value)}
                      placeholder="name@example.com"
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">WhatsApp recipients</span>
                    <input
                      value={deliveryWhatsAppRecipients}
                      onChange={(event) => setDeliveryWhatsAppRecipients(event.target.value)}
                      placeholder="+628..."
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                    />
                  </label>
                  <Button
                    type="submit"
                    disabled={!canDispatchDelivery || deliveryMutation.isPending}
                    className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
                  >
                    {deliveryMutation.isPending ? "Sending..." : "Send Credential"}
                  </Button>
                </form>
                <div className="mt-4 space-y-2">
                  {detailDeliveries.map((delivery) => (
                    <div key={delivery.id} className="rounded-[6px] border border-[#eceef2] px-3 py-3">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-[#2f3037]">{delivery.channels?.join(", ") || delivery.provider || delivery.id}</p>
                          <p className="mt-1 text-xs text-[#6f717c]">
                            {statusText(delivery.status)} · attempt {delivery.attempt ?? 1} · {detailDateLabel(delivery.triggered_at)}
                          </p>
                        </div>
                        {delivery.retryable ? (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={retryDeliveryMutation.isPending}
                            onClick={() => retryDeliveryMutation.mutate(delivery.id)}
                            className="h-8 rounded-[6px] px-2"
                          >
                            <RotateCwIcon className="mr-1 size-3.5" />
                            Retry
                          </Button>
                        ) : null}
                      </div>
                      {delivery.reason || delivery.provider_error ? (
                        <p className="mt-2 text-xs text-[#8a5a00]">{delivery.provider_error || delivery.reason}</p>
                      ) : null}
                    </div>
                  ))}
                  {!deliveriesQuery.isPending && detailDeliveries.length === 0 ? (
                    <p className="text-sm text-[#6f717c]">No deliveries.</p>
                  ) : null}
                </div>
              </section>

              <section className="rounded-[6px] border border-[#d9dbe3] p-4">
                <div className="flex items-center justify-between gap-3 border-b border-[#eceef2] pb-4">
                  <div>
                    <h3 className="text-sm font-semibold text-[#17171c]">{t("kisi.credentials.physicalCard")}</h3>
                    <p className="mt-1 text-xs text-[#6f717c]">
                      {physicalTasksQuery.isPending
                        ? "Loading tasks..."
                        : `${detailPhysicalTasks.length} tasks · ${availablePhysicalInventory.length} available cards · ${physicalVendors.length} vendors`}
                    </p>
                  </div>
                  <ClipboardListIcon className="size-4 text-[#6f717c]" />
                </div>
                <form
                  className="mt-4 space-y-3"
                  onSubmit={(event) => {
                    event.preventDefault()
                    physicalTaskMutation.mutate()
                  }}
                >
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Task</span>
                      <select
                        value={physicalTaskType}
                        onChange={(event) => setPhysicalTaskType(event.target.value as WalletPhysicalCardTask["task_type"])}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="issue">Issue</option>
                        <option value="reissue">Reissue</option>
                        <option value="loss_report">Loss report</option>
                      </select>
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Inventory card</span>
                      <select
                        value={physicalInventoryID}
                        onChange={(event) => {
                          const nextID = event.target.value
                          setPhysicalInventoryID(nextID)
                          const item = availablePhysicalInventory.find((candidate) => candidate.id === nextID)
                          if (item) {
                            setPhysicalCardNumber(item.card_number)
                            setPhysicalVendorID(item.vendor_id ?? "")
                          }
                        }}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="">Manual card number</option>
                        {availablePhysicalInventory.map((item) => (
                          <option key={item.id} value={item.id}>
                            {item.card_number} · {item.vendor_name || "No vendor"}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Card number</span>
                      <input
                        value={physicalCardNumber}
                        list="physical-card-inventory-options"
                        onChange={(event) => {
                          const nextCardNumber = event.target.value
                          setPhysicalCardNumber(nextCardNumber)
                          const item = availablePhysicalInventory.find((candidate) => candidate.card_number === nextCardNumber)
                          setPhysicalInventoryID(item?.id ?? "")
                          if (item?.vendor_id) {
                            setPhysicalVendorID(item.vendor_id)
                          }
                        }}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
                      />
                      <datalist id="physical-card-inventory-options">
                        {availablePhysicalInventory.map((item) => (
                          <option key={item.id} value={item.card_number} />
                        ))}
                      </datalist>
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Vendor</span>
                      <select
                        value={physicalVendorID}
                        onChange={(event) => setPhysicalVendorID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="">No vendor</option>
                        {physicalVendors.map((vendor) => (
                          <option key={vendor.id} value={vendor.id}>
                            {vendor.name} · {vendor.provider}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Note</span>
                    <input
                      value={physicalTaskNote}
                      onChange={(event) => setPhysicalTaskNote(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
                    />
                  </label>
                  <Button
                    type="submit"
                    disabled={!canCreatePhysicalTask || physicalTaskMutation.isPending}
                    className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
                  >
                    {physicalTaskMutation.isPending ? "Creating..." : "Create Task"}
                  </Button>
                </form>
                <form
                  className="mt-5 space-y-3 rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-3"
                  onSubmit={(event) => {
                    event.preventDefault()
                    physicalInventoryScanMutation.mutate()
                  }}
                >
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scanned UID</span>
                      <input
                        value={physicalScanUID}
                        onChange={(event) => setPhysicalScanUID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Card number</span>
                      <input
                        value={physicalScanCardNumber}
                        onChange={(event) => setPhysicalScanCardNumber(event.target.value)}
                        placeholder="Defaults to UID"
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Reader ID</span>
                      <input
                        value={physicalScanReaderID}
                        onChange={(event) => setPhysicalScanReaderID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                  </div>
                  <Button
                    type="submit"
                    variant="outline"
                    disabled={physicalInventoryScanMutation.isPending || !physicalScanUID.trim()}
                    className="h-9 rounded-[6px]"
                  >
                    {physicalInventoryScanMutation.isPending ? "Scanning..." : "Scan Inventory"}
                  </Button>
                </form>
                <form
                  className="mt-5 space-y-3 rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-3"
                  onSubmit={(event) => {
                    event.preventDefault()
                    physicalInventoryImportMutation.mutate()
                  }}
                >
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Inventory CSV</span>
                    <textarea
                      value={physicalInventoryCSV}
                      onChange={(event) => setPhysicalInventoryCSV(event.target.value)}
                      placeholder="card_number,uid,vendor_id,status"
                      rows={3}
                      className="w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 py-2 text-sm text-[#2f3037] placeholder:text-[#9a9ca7]"
                    />
                  </label>
                  <Button
                    type="submit"
                    variant="outline"
                    disabled={physicalInventoryImportMutation.isPending || !physicalInventoryCSV.trim()}
                    className="h-9 rounded-[6px]"
                  >
                    {physicalInventoryImportMutation.isPending ? "Importing..." : "Import Inventory"}
                  </Button>
                </form>
                <div className="mt-5 space-y-3 rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-[#17171c]">Inventory governance</p>
                      <p className="mt-1 text-xs text-[#6f717c]">
                        {physicalInventoryCounts.total} total · {physicalInventoryCounts.available} available ·{" "}
                        {physicalInventoryCounts.frozen} frozen · {physicalInventoryCounts.scrapped} scrapped
                      </p>
                    </div>
                    <div className="grid grid-cols-1 gap-2 sm:grid-cols-[160px_auto_auto]">
                      <select
                        value={physicalInventoryBatchStatus}
                        onChange={(event) =>
                          setPhysicalInventoryBatchStatus(event.target.value as WalletPhysicalCardInventoryGovernanceStatus)
                        }
                        className="h-9 rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                        aria-label="Batch inventory target status"
                      >
                        <option value="available">Return</option>
                        <option value="frozen">Freeze</option>
                        <option value="scrapped">Scrap</option>
                      </select>
                      <Button
                        type="button"
                        variant="outline"
                        disabled={
                          selectedPhysicalInventoryCount === 0 ||
                          physicalInventoryBatchStatusMutation.isPending ||
                          physicalInventoryStatusMutation.isPending
                        }
                        onClick={requestPhysicalInventoryBatchStatus}
                        className="h-9 rounded-[6px]"
                      >
                        Apply selected
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        disabled={selectedPhysicalInventoryCount === 0}
                        onClick={() => setSelectedPhysicalInventoryIDs([])}
                        className="h-9 rounded-[6px] text-[#6f717c] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                      >
                        Clear
                      </Button>
                    </div>
                  </div>
                  <p className="text-xs text-[#6f717c]">
                    {selectedPhysicalInventoryCount} selected. Reserved and issued cards must continue through physical card tasks.
                  </p>
                  <div className="overflow-hidden rounded-[6px] border border-[#e1e3e8] bg-white">
                    <table className="w-full table-fixed text-left text-sm">
                      <thead className="bg-[#fbfbfc] text-xs font-semibold uppercase text-[#6f717c]">
                        <tr>
                          <th className="w-10 px-3 py-2"> </th>
                          <th className="px-3 py-2">Card</th>
                          <th className="w-28 px-3 py-2">{t("common.status")}</th>
                          <th className="hidden px-3 py-2 sm:table-cell">Vendor</th>
                          <th className="w-12 px-3 py-2 text-right"> </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[#eceef2]">
                        {listedPhysicalInventory.map((item) => {
                          const busy = physicalInventoryStatusMutation.isPending || physicalInventoryBatchStatusMutation.isPending
                          const canBatchSelect = canTransitionPhysicalInventoryStatus(item.status, physicalInventoryBatchStatus)
                          return (
                            <tr key={item.id}>
                              <td className="px-3 py-2">
                                <input
                                  type="checkbox"
                                  aria-label={`Select inventory ${item.card_number}`}
                                  checked={selectedPhysicalInventoryIDSet.has(item.id)}
                                  disabled={busy || !canBatchSelect}
                                  title={canBatchSelect ? undefined : `Cannot move ${statusText(item.status)} to ${statusText(physicalInventoryBatchStatus)}`}
                                  onChange={() => togglePhysicalInventorySelection(item.id)}
                                  className="size-4 rounded border-[#d9dbe3]"
                                />
                              </td>
                              <td className="min-w-0 px-3 py-2">
                                <p className="truncate font-medium text-[#2f3037]">{item.card_number}</p>
                                <p className="truncate text-xs text-[#6f717c]">{item.uid || item.id}</p>
                              </td>
                              <td className="px-3 py-2">
                                <StatusDot tone={physicalInventoryStatusTone(item.status)} label={statusText(item.status)} />
                              </td>
                              <td className="hidden min-w-0 px-3 py-2 sm:table-cell">
                                <p className="truncate text-[#2f3037]">{item.vendor_name || "No vendor"}</p>
                                {item.active_task_id || item.assigned_pass_id ? (
                                  <p className="truncate text-xs text-[#6f717c]">
                                    {item.active_task_id || item.assigned_pass_id}
                                  </p>
                                ) : null}
                              </td>
                              <td className="px-3 py-2 text-right">
                                <RowActionsMenu
                                  label={`Inventory actions for ${item.card_number}`}
                                  items={[
                                    {
                                      id: "available",
                                      label: "Return",
                                      icon: RotateCcwIcon,
                                      disabled: busy || !canTransitionPhysicalInventoryStatus(item.status, "available"),
                                      onSelect: () => requestPhysicalInventoryStatus({ item, status: "available" }),
                                    },
                                    {
                                      id: "frozen",
                                      label: "Freeze",
                                      icon: SnowflakeIcon,
                                      disabled: busy || !canTransitionPhysicalInventoryStatus(item.status, "frozen"),
                                      onSelect: () => requestPhysicalInventoryStatus({ item, status: "frozen" }),
                                    },
                                    {
                                      id: "scrapped",
                                      label: "Scrap",
                                      icon: Trash2Icon,
                                      destructive: true,
                                      disabled: busy || !canTransitionPhysicalInventoryStatus(item.status, "scrapped"),
                                      onSelect: () => requestPhysicalInventoryStatus({ item, status: "scrapped" }),
                                    },
                                  ]}
                                />
                              </td>
                            </tr>
                          )
                        })}
                        {!physicalInventoryQuery.isPending && listedPhysicalInventory.length === 0 ? (
                          <tr>
                            <td colSpan={5} className="px-3 py-5 text-center text-sm text-[#6f717c]">
                              No inventory cards.
                            </td>
                          </tr>
                        ) : null}
                      </tbody>
                    </table>
                  </div>
                  {physicalInventory.length > listedPhysicalInventory.length ? (
                    <p className="text-xs text-[#6f717c]">
                      Showing {listedPhysicalInventory.length} of {physicalInventory.length} inventory cards.
                    </p>
                  ) : null}
                </div>
                <div className="mt-4 space-y-2">
                  {detailPhysicalTasks.map((task) => {
                    const nextStatuses = nextPhysicalTaskStatuses(task)
                    return (
                      <div key={task.id} className="rounded-[6px] border border-[#eceef2] px-3 py-3">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                          <div className="min-w-0">
                            <p className="truncate text-sm font-semibold text-[#2f3037]">
                              {statusText(task.task_type)} · {statusText(task.status)}
                            </p>
                            <p className="mt-1 text-xs text-[#6f717c]">
                              {task.card_number || "No card number"} · {detailDateLabel(task.updated_at)}
                            </p>
                            {task.vendor_name || task.inventory_id ? (
                              <p className="mt-1 text-xs text-[#6f717c]">
                                {task.vendor_name || "No vendor"} · {task.inventory_id || "manual card"}
                              </p>
                            ) : null}
                            {task.note ? <p className="mt-2 text-xs text-[#6f717c]">{task.note}</p> : null}
                          </div>
                          {nextStatuses.length > 0 ? (
                            <div className="flex flex-wrap gap-2">
                              {nextStatuses.map((status) => (
                                <Button
                                  key={status}
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  disabled={physicalTaskStatusMutation.isPending}
                                  onClick={() => requestPhysicalTaskStatus({ task, status })}
                                  className="h-8 rounded-[6px] px-2"
                                >
                                  {statusText(status)}
                                </Button>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      </div>
                    )
                  })}
                  {!physicalTasksQuery.isPending && detailPhysicalTasks.length === 0 ? (
                    <p className="text-sm text-[#6f717c]">No physical card tasks.</p>
                  ) : null}
                </div>
              </section>

              <SheetFooter className="-mx-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    disabled={credentialActionMutation.isPending || detailCard.status === "activated" || detailCard.status === "revoked"}
                    onClick={() => requestCredentialAction({ id: detailCard.id, action: "activate", label: detailCard.card_number || detailCard.id })}
                    className="rounded-[6px]"
                  >
                    Activate
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    disabled={credentialActionMutation.isPending || detailCard.status !== "activated"}
                    onClick={() => requestCredentialAction({ id: detailCard.id, action: "deactivate", label: detailCard.card_number || detailCard.id })}
                    className="rounded-[6px]"
                  >
                    Deactivate
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    disabled={credentialActionMutation.isPending || detailCard.status === "unassigned" || detailCard.status === "revoked"}
                    onClick={() => requestCredentialAction({ id: detailCard.id, action: "deassign", label: detailCard.card_number || detailCard.id })}
                    className="rounded-[6px] text-[#6f717c] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                  >
                    Deassign
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    disabled={credentialActionMutation.isPending || detailCard.status === "revoked"}
                    onClick={() => requestCredentialAction({ id: detailCard.id, action: "revoke", label: detailCard.card_number || detailCard.id })}
                    className="rounded-[6px] text-[#bd2f2f] hover:bg-[#fff5f5] hover:text-[#9f1d1d]"
                  >
                    Revoke
                  </Button>
                </div>
              </SheetFooter>
            </div>
          ) : null}
        </SheetContent>
      </Sheet>
      <ConfirmActionDialog
        open={Boolean(physicalTaskStatusTarget)}
        onOpenChange={(open) => {
          if (!physicalTaskStatusMutation.isPending && !open) {
            setPhysicalTaskStatusTarget(null)
          }
        }}
        title={physicalTaskStatusDialogTitle(physicalTaskStatusTarget?.status)}
        description={
          <>
            {physicalTaskStatusDescription(physicalTaskStatusTarget?.status)} Target:{" "}
            <span className="font-semibold text-[#17171c]">
              {physicalTaskStatusTarget?.task.card_number || physicalTaskStatusTarget?.task.id || "this task"}
            </span>.
          </>
        }
        confirmLabel={statusText(physicalTaskStatusTarget?.status ?? "update")}
        pending={physicalTaskStatusMutation.isPending}
        disabled={!physicalTaskStatusTarget}
        destructive={physicalTaskStatusTarget?.status === "reported_lost" || physicalTaskStatusTarget?.status === "cancelled"}
        onConfirm={() => {
          if (physicalTaskStatusTarget) {
            physicalTaskStatusMutation.mutate(physicalTaskStatusTarget)
          }
        }}
      />
      <ConfirmActionDialog
        open={Boolean(physicalInventoryStatusTarget)}
        onOpenChange={(open) => {
          if (!physicalInventoryStatusMutation.isPending && !open) {
            setPhysicalInventoryStatusTarget(null)
          }
        }}
        title={physicalInventoryStatusDialogTitle(physicalInventoryStatusTarget?.status)}
        description={
          <>
            {physicalInventoryStatusDescription(physicalInventoryStatusTarget?.status)} Target:{" "}
            <span className="font-semibold text-[#17171c]">
              {physicalInventoryStatusTarget?.item.card_number || "this inventory card"}
            </span>.
          </>
        }
        confirmLabel={statusText(physicalInventoryStatusTarget?.status ?? "update")}
        pending={physicalInventoryStatusMutation.isPending}
        disabled={!physicalInventoryStatusTarget}
        destructive={physicalInventoryStatusTarget?.status === "scrapped"}
        onConfirm={() => {
          if (physicalInventoryStatusTarget) {
            physicalInventoryStatusMutation.mutate(physicalInventoryStatusTarget)
          }
        }}
      />
      <ConfirmActionDialog
        open={Boolean(physicalInventoryBatchStatusTarget)}
        onOpenChange={(open) => {
          if (!physicalInventoryBatchStatusMutation.isPending && !open) {
            setPhysicalInventoryBatchStatusTarget(null)
          }
        }}
        title={physicalInventoryStatusDialogTitle(physicalInventoryBatchStatusTarget?.status)}
        description={
          <>
            {physicalInventoryStatusDescription(physicalInventoryBatchStatusTarget?.status)} Target:{" "}
            <span className="font-semibold text-[#17171c]">
              {physicalInventoryBatchStatusTarget?.inventoryIDs.length ?? 0} selected inventory cards
            </span>.
          </>
        }
        confirmLabel={statusText(physicalInventoryBatchStatusTarget?.status ?? "update")}
        pending={physicalInventoryBatchStatusMutation.isPending}
        disabled={!physicalInventoryBatchStatusTarget}
        destructive={physicalInventoryBatchStatusTarget?.status === "scrapped"}
        onConfirm={() => {
          if (physicalInventoryBatchStatusTarget) {
            physicalInventoryBatchStatusMutation.mutate(physicalInventoryBatchStatusTarget)
          }
        }}
      />
      <ConfirmActionDialog
        open={Boolean(credentialActionTarget)}
        onOpenChange={(open) => {
          if (!credentialActionMutation.isPending && !open) {
            setCredentialActionTarget(null)
          }
        }}
        title={credentialActionTitle(credentialActionTarget?.action)}
        description={
          <>
            {credentialActionDescription(credentialActionTarget?.action)} Target:{" "}
            <span className="font-semibold text-[#17171c]">{credentialActionTarget?.label ?? "this credential"}</span>.
          </>
        }
        confirmLabel={credentialActionConfirmLabel(credentialActionTarget?.action)}
        pending={credentialActionMutation.isPending}
        disabled={!credentialActionTarget}
        destructive={credentialActionTarget?.action !== "activate"}
        onConfirm={() => {
          if (credentialActionTarget) {
            credentialActionMutation.mutate({
              id: credentialActionTarget.id,
              action: credentialActionTarget.action,
            })
          }
        }}
      />
    </PageFrame>
  )
}
