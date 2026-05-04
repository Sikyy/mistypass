import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { WifiIcon, LockOpenIcon, CheckCircleIcon, XCircleIcon } from "lucide-react"
import {
  southboundTestConnection,
  southboundUnlock,
  type SouthboundProvider,
  type CurrentUser,
} from "@/lib/api"

type SouthboundPageProps = {
  token: string
  viewer: CurrentUser
}

export function SouthboundPage({ token, viewer }: SouthboundPageProps) {
  const [provider, setProvider] = useState<SouthboundProvider>("hikvision")
  const [host, setHost] = useState("")
  const [port, setPort] = useState("80")
  const [username, setUsername] = useState("admin")
  const [password, setPassword] = useState("")
  const [deviceID, setDeviceID] = useState("device_001")
  const [doorIndex, setDoorIndex] = useState("1")
  const [result, setResult] = useState<{ type: "success" | "error"; message: string } | null>(null)

  const testMutation = useMutation({
    mutationFn: () =>
      southboundTestConnection(token, provider, {
        host,
        port: parseInt(port) || 80,
        username,
        password,
      }),
    onSuccess: (data) => {
      if (data.reachable) {
        setResult({ type: "success", message: `${provider} device reachable` })
      } else {
        setResult({ type: "error", message: data.error || "Device unreachable" })
      }
    },
    onError: (e) => setResult({ type: "error", message: e.message }),
  })

  const unlockMutation = useMutation({
    mutationFn: () =>
      southboundUnlock(token, provider, deviceID, {
        host,
        port: parseInt(port) || 80,
        username,
        password,
        door_index: parseInt(doorIndex) || 1,
      }),
    onSuccess: (data) => {
      setResult({ type: "success", message: `Door unlocked (${data.status})` })
    },
    onError: (e) => setResult({ type: "error", message: e.message }),
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-[#1a1c23]">Southbound Gateway</h1>
        <p className="mt-1 text-sm text-[#6f717c]">
          Direct control of third-party door controllers (Hikvision ISAPI / ZKTeco)
        </p>
      </div>

      {/* Device Configuration */}
      <div className="rounded-lg border border-[#eceef2] bg-white p-6">
        <h2 className="mb-4 text-lg font-medium">Device Connection</h2>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Provider</label>
            <select
              value={provider}
              onChange={(e) => setProvider(e.target.value as SouthboundProvider)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
            >
              <option value="hikvision">Hikvision (ISAPI)</option>
              <option value="zkteco">ZKTeco (CGI/Push)</option>
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Device ID</label>
            <input
              value={deviceID}
              onChange={(e) => setDeviceID(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="device_001"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Host / IP</label>
            <input
              value={host}
              onChange={(e) => setHost(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="192.168.1.64"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Port</label>
            <input
              value={port}
              onChange={(e) => setPort(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="80"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Username</label>
            <input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="admin"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="password"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-[#6f717c]">Door Index</label>
            <input
              value={doorIndex}
              onChange={(e) => setDoorIndex(e.target.value)}
              className="w-full rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
              placeholder="1"
            />
          </div>
        </div>

        {/* Actions */}
        <div className="mt-6 flex gap-3">
          <button
            onClick={() => testMutation.mutate()}
            disabled={!host || testMutation.isPending}
            className="flex items-center gap-2 rounded-md border border-[#d9dbe3] px-4 py-2 text-sm hover:bg-[#f5f6f8] disabled:opacity-50"
          >
            <WifiIcon className="size-4" />
            {testMutation.isPending ? "Testing..." : "Test Connection"}
          </button>
          <button
            onClick={() => unlockMutation.mutate()}
            disabled={!host || unlockMutation.isPending}
            className="flex items-center gap-2 rounded-md bg-[#1a1c23] px-4 py-2 text-sm text-white hover:bg-[#2d2f38] disabled:opacity-50"
          >
            <LockOpenIcon className="size-4" />
            {unlockMutation.isPending ? "Unlocking..." : "Unlock Door"}
          </button>
        </div>

        {/* Result */}
        {result && (
          <div
            className={`mt-4 flex items-center gap-2 rounded-md p-3 text-sm ${
              result.type === "success"
                ? "bg-green-50 text-green-800"
                : "bg-red-50 text-red-800"
            }`}
          >
            {result.type === "success" ? (
              <CheckCircleIcon className="size-4" />
            ) : (
              <XCircleIcon className="size-4" />
            )}
            {result.message}
          </div>
        )}
      </div>

      {/* Provider info */}
      <div className="rounded-lg border border-[#eceef2] bg-white p-6">
        <h2 className="mb-3 text-lg font-medium">Provider Reference</h2>
        <div className="grid grid-cols-2 gap-6 text-sm">
          <div>
            <h3 className="font-medium">Hikvision (ISAPI)</h3>
            <ul className="mt-2 space-y-1 text-[#6f717c]">
              <li>Protocol: HTTP REST + Digest Auth</li>
              <li>Default Port: 80 (HTTP) / 443 (HTTPS)</li>
              <li>Unlock: PUT /ISAPI/AccessControl/RemoteControl/door/N</li>
              <li>Events: GET /ISAPI/Event/notification/alertStream</li>
            </ul>
          </div>
          <div>
            <h3 className="font-medium">ZKTeco (CGI)</h3>
            <ul className="mt-2 space-y-1 text-[#6f717c]">
              <li>Protocol: HTTP CGI + Basic Auth</li>
              <li>Default Port: 80</li>
              <li>Unlock: POST /cgi-bin/accessControl.cgi?action=openDoor</li>
              <li>Events: Push Protocol (device → platform)</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}
