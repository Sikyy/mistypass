import { useQuery } from "@tanstack/react-query"
import { NetworkIcon, ServerIcon, WifiIcon } from "lucide-react"

import { PageFrame, KpiCard, StatusDot } from "@/components/mistyislet/primitives"
import { getNetworkTopology, type CurrentUser } from "@/lib/api"

export function NetworkTopologyPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const topologyQuery = useQuery({
    queryKey: ["network-topology"],
    queryFn: () => getNetworkTopology(token),
    refetchInterval: 30_000,
  })

  const topology = topologyQuery.data
  const loading = topologyQuery.isPending

  return (
    <PageFrame
      breadcrumbs={["Home", "Network"]}
      title="Network Topology"
      description="Live view of gateways and connected devices across your properties."
    >
      {/* Summary bar */}
      <section>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard
            label="Gateways"
            value={String(topology?.total_gateways ?? 0)}
            note={`${topology?.online_gateways ?? 0} online`}
            icon={NetworkIcon}
            tone="info"
            loading={loading}
          />
          <KpiCard
            label="Devices"
            value={String(topology?.total_devices ?? 0)}
            note={`${topology?.online_devices ?? 0} online`}
            icon={ServerIcon}
            tone="info"
            loading={loading}
          />
          <KpiCard
            label="Online"
            value={String((topology?.online_gateways ?? 0) + (topology?.online_devices ?? 0))}
            note="Gateways + devices responding"
            icon={WifiIcon}
            tone="success"
            loading={loading}
          />
          <KpiCard
            label="Offline"
            value={String((topology?.offline_gateways ?? 0) + (topology?.offline_devices ?? 0))}
            note="Needs attention"
            icon={WifiIcon}
            tone={((topology?.offline_gateways ?? 0) + (topology?.offline_devices ?? 0)) > 0 ? "danger" : "success"}
            loading={loading}
          />
        </div>
      </section>

      {/* Topology list */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-[#17171c]">Gateway Topology</h2>
        {loading ? (
          <p className="py-8 text-center text-sm text-[#6f717c]">Loading topology...</p>
        ) : !topology || topology.gateways.length === 0 ? (
          <div className="rounded-[6px] border border-[#e1e3e8] bg-white px-5 py-8 text-center text-sm text-[#6f717c]">
            No gateways found.
          </div>
        ) : (
          <div className="space-y-4">
            {topology.gateways.map((gw) => (
              <div key={gw.id} className="rounded-[6px] border border-[#e1e3e8] bg-white">
                {/* Gateway header */}
                <div className="flex items-center gap-4 border-b border-[#eceef2] px-5 py-4">
                  <div className="flex size-10 shrink-0 items-center justify-center rounded-[14px] bg-[#fbfbfc] text-[#6f717c] ring-1 ring-[#eceef2]">
                    <NetworkIcon className="size-5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-[#17171c]">
                      {gw.serial_number}
                    </p>
                    <p className="text-xs text-[#6f717c]">{gw.building_name}</p>
                  </div>
                  <StatusDot
                    tone={gw.status === "online" ? "success" : "danger"}
                    label={gw.status}
                  />
                </div>

                {/* Connected devices */}
                {gw.devices.length === 0 ? (
                  <p className="px-5 py-4 text-sm text-[#6f717c]">No connected devices.</p>
                ) : (
                  <div className="divide-y divide-[#eceef2]">
                    {gw.devices.map((device) => (
                      <div key={device.id} className="flex items-center gap-4 px-5 py-3 pl-14">
                        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-[#f1f2f5] text-[#6f717c]">
                          <ServerIcon className="size-4" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium text-[#2f3037]">
                            {device.name}
                          </p>
                          <p className="text-xs text-[#9a9ca7]">{device.kind}</p>
                        </div>
                        <span className="flex items-center gap-1.5 text-xs text-[#6f717c]">
                          <span
                            className={`size-2 rounded-full ${device.status === "online" ? "bg-[#35a853]" : "bg-[#d93025]"}`}
                          />
                          {device.status}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </PageFrame>
  )
}
