type AccessGrantOverviewCardsProps = {
  activeCount: number
  expiringSoonCount: number
  visitorCount: number
  expiredCount: number
  onShowActive: () => void
  onShowExpiringSoon: () => void
  onShowVisitors: () => void
  onShowExpired: () => void
}

export function AccessGrantOverviewCards({
  activeCount,
  expiringSoonCount,
  visitorCount,
  expiredCount,
  onShowActive,
  onShowExpiringSoon,
  onShowVisitors,
  onShowExpired,
}: AccessGrantOverviewCardsProps) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowActive}
      >
        <p className="mp-kpi-note">当前有效</p>
        <p className="mt-1 text-sm font-medium">{activeCount} 条</p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowExpiringSoon}
      >
        <p className="mp-kpi-note">24 小时内到期</p>
        <p className="mt-1 text-sm font-medium">{expiringSoonCount} 条</p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowVisitors}
      >
        <p className="mp-kpi-note">访客授权</p>
        <p className="mt-1 text-sm font-medium">{visitorCount} 条</p>
      </button>
      <button
        type="button"
        className="rounded-lg border bg-muted/10 px-3 py-3 text-left transition-colors hover:bg-muted/20"
        onClick={onShowExpired}
      >
        <p className="mp-kpi-note">已到期</p>
        <p className="mt-1 text-sm font-medium">{expiredCount} 条</p>
      </button>
    </div>
  )
}
