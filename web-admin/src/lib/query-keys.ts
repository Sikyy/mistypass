/**
 * Centralized React Query key factory.
 * Prevents typos and ensures consistent cache invalidation.
 *
 * Usage:
 *   queryKey: queryKeys.guests(tenantID)
 *   queryClient.invalidateQueries({ queryKey: queryKeys.guests._base })
 */

const k = <T extends readonly unknown[]>(...parts: T) => parts

// Helper: create a namespaced key with a _base prefix for invalidation
function ns<TBase extends string>(base: TBase) {
  return {
    _base: [base] as const,
    _key: (...args: string[]) => [base, ...args] as const,
  }
}

export const queryKeys = {
  // --- Navigation & Shell ---
  navigationBuildings: (...args: string[]) => k("kisi-navigation-buildings", ...args),
  roleScopeBuildings: (...args: string[]) => k("role-scope-buildings", ...args),
  homeSummary: (...args: string[]) => k("kisi-home-summary", ...args),
  resourceSummary: ns("kisi-resource-summary"),

  // --- Places & Spaces ---
  spacesTenants: (...args: string[]) => k("spaces-tenants", ...args),

  // --- Groups ---
  groupLinks: ns("reference-group-links"),

  // --- Credentials ---
  cardDetail: ns("credential-card-detail"),
  cardAssignmentDetail: ns("credential-card-assignment-detail"),
  walletJobs: ns("credential-wallet-jobs"),
  physicalCardTasks: ns("credential-physical-card-tasks"),
  physicalCardVendors: ns("credential-physical-card-vendors"),
  physicalCardInventory: ns("credential-physical-card-inventory"),
  deliveries: ns("credential-deliveries"),

  // --- Reports ---
  reports: ns("reference-reports"),
  scheduledReports: ns("reference-scheduled-reports"),
  reportSchedules: ns("report-schedules"),

  // --- Analytics ---
  accessSummary: (...args: string[]) => k("analytics-access-summary", ...args),
  doorActivity: (...args: string[]) => k("analytics-door-activity", ...args),
  alarmMetrics: (...args: string[]) => k("analytics-alarm-metrics", ...args),

  // --- Network ---
  networkTopology: ns("network-topology"),

  // --- Alarms ---
  alarms: ns("alarms-page"),
  alarmSchedules: ns("alarm-schedules"),
  alarmScheduleCalendar: ns("alarm-schedule-calendar"),

  // --- Visitors ---
  guests: ns("guests"),

  // --- Events & Audit ---
  events: ns("events-page"),
  auditLogs: ns("audit-logs"),

  // --- Organization ---
  orgSettings: ns("orgSettings"),
  integrations: ns("reference-integrations"),
  integrationDetail: ns("reference-integration-detail"),
  alertPolicies: ns("reference-alert-policies"),

  // --- Access ---
  accessBaseData: ns("access-base-data"),
  accessLinkClaim: ns("access-link-claim"),

  // --- OTA / Firmware ---
  firmwareSummary: ns("ota-firmware-summary"),
  firmwareList: ns("ota-firmware-list"),

  // --- OTA / Rollouts ---
  rolloutList: ns("ota-rollout-list"),
  rolloutDetail: ns("ota-rollout-detail"),
} as const
