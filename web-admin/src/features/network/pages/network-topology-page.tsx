import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  CloudIcon,
  NetworkIcon,
  ServerIcon,
  WifiIcon,
  RadioIcon,
  ScanLineIcon,
  CircuitBoardIcon,
  CpuIcon,
  ActivityIcon,
  RefreshCwIcon,
} from "lucide-react"
import type { LucideIcon } from "lucide-react"

import { PageFrame, KpiCard, StatusDot } from "@/components/mistyislet/primitives"
import { cn } from "@/lib/utils"
import { queryKeys } from "@/lib/query-keys"
import { getNetworkTopology, type CurrentUser, type NetworkTopology } from "@/lib/api"

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type TopologyNode = NetworkTopology["nodes"][number]
type TopologyEdge = NetworkTopology["edges"][number]

/** Resolve the icon for each node type. */
function nodeIcon(type: string): LucideIcon {
  switch (type) {
    case "cloud":
      return CloudIcon
    case "gateway":
      return NetworkIcon
    case "reader":
      return ScanLineIcon
    case "controller":
      return CpuIcon
    case "relay":
      return CircuitBoardIcon
    case "sensor":
      return ActivityIcon
    default:
      return ServerIcon
  }
}

/** Human-friendly protocol label for edge connections. */
function protocolLabel(protocol: string): string {
  switch (protocol) {
    case "https+nats":
      return "HTTPS / NATS"
    case "osdp_v2":
      return "OSDP v2"
    case "rs485":
      return "RS-485"
    case "ble":
      return "BLE"
    case "wiegand_26":
      return "Wiegand 26"
    case "wiegand_34":
      return "Wiegand 34"
    case "usb":
      return "USB"
    default:
      return protocol || "Direct"
  }
}

/** Format an ISO timestamp into a short relative or absolute string. */
function formatLastSeen(iso: string | undefined): string {
  if (!iso) return "Never"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return "Unknown"
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60_000) return "Just now"
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

// ---------------------------------------------------------------------------
// Build tree from flat nodes + edges
// ---------------------------------------------------------------------------

type TreeNode = {
  node: TopologyNode
  edge?: TopologyEdge
  children: TreeNode[]
}

function buildTree(nodes: TopologyNode[], edges: TopologyEdge[]): TreeNode[] {
  const nodeMap = new Map<string, TopologyNode>()
  for (const n of nodes) nodeMap.set(n.id, n)

  // childIDs from edges: source -> target[]
  const childrenOf = new Map<string, { targetID: string; edge: TopologyEdge }[]>()
  const hasParent = new Set<string>()
  for (const e of edges) {
    if (!childrenOf.has(e.source)) childrenOf.set(e.source, [])
    childrenOf.get(e.source)!.push({ targetID: e.target, edge: e })
    hasParent.add(e.target)
  }

  function subtree(id: string, edge?: TopologyEdge): TreeNode | undefined {
    const node = nodeMap.get(id)
    if (!node) return undefined
    const kids = childrenOf.get(id) ?? []
    const children: TreeNode[] = []
    for (const k of kids) {
      const child = subtree(k.targetID, k.edge)
      if (child) children.push(child)
    }
    return { node, edge, children }
  }

  // Roots are nodes with no incoming edge
  const roots: TreeNode[] = []
  for (const n of nodes) {
    if (!hasParent.has(n.id)) {
      const t = subtree(n.id)
      if (t) roots.push(t)
    }
  }
  return roots
}

// ---------------------------------------------------------------------------
// Device card component
// ---------------------------------------------------------------------------

function DeviceCard({
  node,
  edge,
  depth,
}: {
  node: TopologyNode
  edge?: TopologyEdge
  depth: number
}) {
  const Icon = nodeIcon(node.type)
  const isOnline = node.status === "online"
  const isCloud = node.type === "cloud"
  const isGateway = node.type === "gateway"

  // Choose card styling by hierarchy level
  const cardBg = isCloud
    ? "bg-gradient-to-r from-[#f3f4ff] to-white border-[#c5c9ff]"
    : isGateway
      ? "bg-white border-line-default"
      : "bg-surface-page border-line-subtle"

  const iconBg = isCloud
    ? "bg-brand-subtle text-brand"
    : isGateway
      ? "bg-surface-page text-content-subtle ring-1 ring-[#eceef2]"
      : "bg-surface-sunken text-content-subtle"

  const iconSize = isCloud || isGateway ? "size-10" : "size-8"
  const innerIconSize = isCloud || isGateway ? "size-5" : "size-4"

  return (
    <div className={`rounded-[6px] border ${cardBg} px-5 py-4`}>
      <div className="flex items-center gap-4">
        <div
          className={`flex ${iconSize} shrink-0 items-center justify-center rounded-[14px] ${iconBg}`}
        >
          <Icon className={innerIconSize} />
        </div>

        <div className="min-w-0 flex-1">
          <p
            className={`truncate font-medium ${
              isCloud || isGateway
                ? "text-sm font-semibold text-content-heading"
                : "text-sm text-content-body"
            }`}
          >
            {node.label}
          </p>
          <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-content-muted">
            <span className="capitalize">{node.type}</span>
            {node.protocol ? (
              <span className="rounded bg-surface-sunken px-1.5 py-0.5 text-[10px] font-medium text-content-subtle">
                {protocolLabel(node.protocol)}
              </span>
            ) : null}
            {node.ip ? <span>{node.ip}</span> : null}
            {!isCloud ? <span>{formatLastSeen(node.last_seen)}</span> : null}
          </div>
        </div>

        <StatusDot
          tone={isOnline ? "success" : "danger"}
          label={isOnline ? "Online" : "Offline"}
        />
      </div>

      {/* Edge / connection label */}
      {edge && depth > 0 ? (
        <div className="mt-2 ml-14 flex items-center gap-1.5 text-[10px] text-content-muted">
          <RadioIcon className="size-3" />
          <span>via {protocolLabel(edge.protocol)}</span>
        </div>
      ) : null}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Recursive tree renderer
// ---------------------------------------------------------------------------

function TreeLevel({ trees, depth }: { trees: TreeNode[]; depth: number }) {
  if (trees.length === 0) return null
  return (
    <div className={depth === 0 ? "space-y-6" : "mt-3 space-y-3"}>
      {trees.map((tree) => (
        <div key={tree.node.id}>
          {/* Indent child levels to show hierarchy */}
          <div style={{ marginLeft: depth > 0 ? `${Math.min(depth, 3) * 24}px` : undefined }}>
            {/* Connection line for child nodes */}
            {depth > 0 ? (
              <div className="mb-1 ml-4 flex items-center gap-1">
                <div className="h-px w-4 bg-[#d9dbe3]" />
                <div className="h-3 w-px bg-[#d9dbe3]" />
              </div>
            ) : null}
            <DeviceCard node={tree.node} edge={tree.edge} depth={depth} />
          </div>
          {tree.children.length > 0 ? (
            <TreeLevel trees={tree.children} depth={depth + 1} />
          ) : null}
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Flat grid fallback -- groups by type
// ---------------------------------------------------------------------------

function FlatGrid({ nodes }: { nodes: TopologyNode[] }) {
  const groups = useMemo(() => {
    const map = new Map<string, TopologyNode[]>()
    for (const n of nodes) {
      const key = n.type
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(n)
    }
    // Sort: cloud first, then gateway, then rest alphabetically
    const order = ["cloud", "gateway", "controller", "reader", "relay", "sensor"]
    return [...map.entries()].sort(([a], [b]) => {
      const ai = order.indexOf(a)
      const bi = order.indexOf(b)
      return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
    })
  }, [nodes])

  return (
    <div className="space-y-6">
      {groups.map(([type, items]) => (
        <div key={type}>
          <h3 className="mb-3 text-sm font-semibold capitalize text-content-heading">
            {type === "cloud" ? "Cloud" : `${type}s`}
            <span className="ml-2 text-xs font-normal text-content-muted">({items.length})</span>
          </h3>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((node) => (
              <DeviceCard key={node.id} node={node} depth={0} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Error state
// ---------------------------------------------------------------------------

function ErrorBanner({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className={cn("mp-alert-danger", "flex items-center gap-3")}>
      <span className="min-w-0 flex-1">{message}</span>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex items-center gap-1.5 rounded-[6px] border border-line-default bg-white px-3 py-1.5 text-xs font-medium text-content-body hover:bg-surface-sunken"
        >
          <RefreshCwIcon className="size-3" />
          Retry
        </button>
      ) : null}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export function NetworkTopologyPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const tenantID = viewer.tenant_id

  const topologyQuery = useQuery({
    queryKey: queryKeys.networkTopology._key(tenantID),
    queryFn: () => getNetworkTopology(token, tenantID),
    refetchInterval: 30_000,
  })

  const topology = topologyQuery.data
  const loading = topologyQuery.isPending
  const error = topologyQuery.error

  const tree = useMemo(
    () => (topology ? buildTree(topology.nodes, topology.edges) : []),
    [topology],
  )

  // Determine whether we got a proper tree structure or just flat nodes
  const hasEdges = (topology?.edges.length ?? 0) > 0

  const summary = topology?.summary
  const onlineTotal = summary?.online ?? 0
  const offlineTotal = summary?.offline ?? 0

  return (
    <PageFrame
      breadcrumbs={["Home", "Network"]}
      title="Network Topology"
      description="Live view of gateways and connected devices across your properties."
      actions={
        <button
          type="button"
          onClick={() => topologyQuery.refetch()}
          disabled={topologyQuery.isFetching}
          className="inline-flex items-center gap-2 rounded-[6px] border border-line-default bg-white px-4 py-2 text-sm font-medium text-content-body hover:bg-surface-sunken disabled:opacity-50"
        >
          <RefreshCwIcon className={`size-4 ${topologyQuery.isFetching ? "animate-spin" : ""}`} />
          Refresh
        </button>
      }
    >
      {/* Summary KPI bar */}
      <section>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard
            label="Gateways"
            value={String(summary?.total_gateways ?? 0)}
            note={`${summary?.total_gateways ?? 0} registered`}
            icon={NetworkIcon}
            tone="info"
            loading={loading}
          />
          <KpiCard
            label="Devices"
            value={String(summary?.total_devices ?? 0)}
            note="Connected peripherals"
            icon={ServerIcon}
            tone="info"
            loading={loading}
          />
          <KpiCard
            label="Online"
            value={String(onlineTotal)}
            note="All nodes responding"
            icon={WifiIcon}
            tone="success"
            loading={loading}
          />
          <KpiCard
            label="Offline"
            value={String(offlineTotal)}
            note={offlineTotal > 0 ? "Needs attention" : "All clear"}
            icon={WifiIcon}
            tone={offlineTotal > 0 ? "danger" : "success"}
            loading={loading}
          />
        </div>
      </section>

      {/* Topology visualization */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-content-heading">Topology Map</h2>

        {error ? (
          <ErrorBanner
            message={`Failed to load topology: ${error.message}`}
            onRetry={() => topologyQuery.refetch()}
          />
        ) : loading ? (
          <div className="rounded-[6px] border border-line-default bg-white px-5 py-12 text-center">
            <div className="mx-auto mb-3 size-8 animate-spin rounded-full border-2 border-line-default border-t-brand" />
            <p className="text-sm text-content-subtle">Loading network topology...</p>
          </div>
        ) : !topology || topology.nodes.length === 0 ? (
          <div className="rounded-[6px] border border-line-default bg-white px-5 py-12 text-center">
            <NetworkIcon className="mx-auto mb-3 size-10 text-[#d9dbe3]" />
            <p className="text-sm font-medium text-content-body">No network devices found</p>
            <p className="mt-1 text-xs text-content-muted">
              Register a gateway to see your network topology here.
            </p>
          </div>
        ) : hasEdges ? (
          <TreeLevel trees={tree} depth={0} />
        ) : (
          <FlatGrid nodes={topology.nodes} />
        )}
      </section>

      {/* Legend */}
      {topology && topology.nodes.length > 0 ? (
        <section>
          <h2 className="mb-3 text-sm font-semibold text-content-heading">Legend</h2>
          <div className="flex flex-wrap gap-x-6 gap-y-2 rounded-[6px] border border-line-default bg-white px-5 py-3">
            {(
              [
                ["cloud", "Cloud"],
                ["gateway", "Gateway"],
                ["reader", "Reader"],
                ["controller", "Controller"],
                ["relay", "Relay"],
                ["sensor", "Sensor"],
              ] as const
            ).map(([type, label]) => {
              const Icon = nodeIcon(type)
              return (
                <span key={type} className="inline-flex items-center gap-1.5 text-xs text-content-subtle">
                  <Icon className="size-3.5" />
                  {label}
                </span>
              )
            })}
            <span className="inline-flex items-center gap-1.5 text-xs text-content-subtle">
              <span className="size-2 rounded-full bg-[#35a853]" />
              Online
            </span>
            <span className="inline-flex items-center gap-1.5 text-xs text-content-subtle">
              <span className="size-2 rounded-full bg-[#d93025]" />
              Offline
            </span>
          </div>
        </section>
      ) : null}
    </PageFrame>
  )
}
