import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"

import {
  type AccessPolicy,
  type EnterpriseEmployee,
  type EnterpriseHRISConnector,
  type EnterpriseHRISPullState,
  type EnterpriseHRISSecret,
  type EnterpriseIDPConfig,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"

type EnterpriseDataQueryResult = {
  employees: EnterpriseEmployee[]
  hrisConnectors: EnterpriseHRISConnector[]
  hrisSecrets: EnterpriseHRISSecret[]
  hrisPullStates: EnterpriseHRISPullState[]
  idpConfig: EnterpriseIDPConfig | null
  userGroups: UserGroup[]
  policies: AccessPolicy[]
  issuedPasses: WalletPassInstance[]
}

type UseEnterpriseEmployeesParams = {
  selectedTenantID: string
  enterpriseData: EnterpriseDataQueryResult | undefined
}

export function useEnterpriseEmployees({
  selectedTenantID,
  enterpriseData,
}: UseEnterpriseEmployeesParams) {
  const [employees, setEmployees] = useState<EnterpriseEmployee[]>([])
  const [hrisConnectors, setHRISConnectors] = useState<EnterpriseHRISConnector[]>([])
  const [hrisSecrets, setHRISSecrets] = useState<EnterpriseHRISSecret[]>([])
  const [hrisPullStates, setHRISPullStates] = useState<EnterpriseHRISPullState[]>([])
  const [idpConfig, setIDPConfig] = useState<EnterpriseIDPConfig | null>(null)
  const [userGroups, setUserGroups] = useState<UserGroup[]>([])
  const [policies, setPolicies] = useState<AccessPolicy[]>([])
  const [issuedPasses, setIssuedPasses] = useState<WalletPassInstance[]>([])

  useEffect(() => {
    const effectiveTenantID = selectedTenantID.trim()
    if (!effectiveTenantID) {
      setEmployees([])
      setHRISConnectors([])
      setHRISSecrets([])
      setHRISPullStates([])
      setIDPConfig(null)
      setUserGroups([])
      setPolicies([])
      setIssuedPasses([])
      return
    }

    if (!enterpriseData) {
      return
    }

    setEmployees(enterpriseData.employees)
    setHRISConnectors(enterpriseData.hrisConnectors)
    setHRISSecrets(enterpriseData.hrisSecrets)
    setHRISPullStates(enterpriseData.hrisPullStates)
    setIDPConfig(enterpriseData.idpConfig)
    setUserGroups(enterpriseData.userGroups)
    setPolicies(enterpriseData.policies)
    setIssuedPasses(enterpriseData.issuedPasses)
  }, [enterpriseData, selectedTenantID])

  const tenantGroups = useMemo(
    () => userGroups.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [selectedTenantID, userGroups]
  )
  const tenantPolicies = useMemo(
    () => policies.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [policies, selectedTenantID]
  )
  const activeEmployeeCount = useMemo(
    () => employees.filter((item) => item.status === "active").length,
    [employees]
  )
  const idpReady = Boolean(idpConfig && idpConfig.status === "active")
  const issuedPassCount = issuedPasses.length

  const primaryEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status === "active" && item.email.trim()) ??
      employees.find((item) => item.status === "active") ??
      employees[0] ??
      null,
    [employees]
  )
  const primaryEmployeeTargetHint = useMemo(() => {
    if (!primaryEmployeeHint) {
      return ""
    }
    return primaryEmployeeHint.email.trim() || primaryEmployeeHint.external_id.trim() || primaryEmployeeHint.id.trim()
  }, [primaryEmployeeHint])
  const deactivatedEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status !== "active" && item.email.trim()) ??
      employees.find((item) => item.status !== "active") ??
      null,
    [employees]
  )
  const primaryGroupHint = useMemo(() => tenantGroups[0]?.name?.trim() || "Common Office Access", [tenantGroups])

  return {
    employees,
    setEmployees,
    hrisConnectors,
    setHRISConnectors,
    hrisSecrets,
    hrisPullStates,
    idpConfig,
    userGroups,
    policies,
    issuedPasses,
    tenantGroups,
    tenantPolicies,
    activeEmployeeCount,
    idpReady,
    issuedPassCount,
    primaryEmployeeHint,
    primaryEmployeeTargetHint,
    deactivatedEmployeeHint,
    primaryGroupHint,
  }
}
