import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

const ACCESS_TOKEN_KEY = "mistypass_admin_access_token"
const REFRESH_TOKEN_KEY = "mistypass_admin_refresh_token"

type StorageLike = {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

function createStorage(initial: Record<string, string> = {}): StorageLike {
  const store = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return store.get(key) ?? null
    },
    setItem(key, value) {
      store.set(key, value)
    },
    removeItem(key) {
      store.delete(key)
    },
  }
}

function installWindow(args?: {
  localStorage?: Record<string, string>
  sessionStorage?: Record<string, string>
}) {
  const target = new EventTarget()
  const windowStub = {
    addEventListener: target.addEventListener.bind(target),
    dispatchEvent: target.dispatchEvent.bind(target),
    localStorage: createStorage(args?.localStorage),
    removeEventListener: target.removeEventListener.bind(target),
    sessionStorage: createStorage(args?.sessionStorage),
  }
  vi.stubGlobal("window", windowStub)
  return windowStub
}

async function importAuthModule() {
  return import("./auth")
}

describe("auth session helpers", () => {
  beforeEach(() => {
    vi.resetModules()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.resetModules()
  })

  it("migrates legacy tokens from localStorage into sessionStorage on first read", async () => {
    const windowStub = installWindow({
      localStorage: {
        [ACCESS_TOKEN_KEY]: "legacy-access",
        [REFRESH_TOKEN_KEY]: "legacy-refresh",
      },
    })
    const { getRefreshToken, getToken } = await importAuthModule()

    expect(getToken()).toBe("legacy-access")
    expect(getRefreshToken()).toBe("legacy-refresh")
    expect(windowStub.sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe("legacy-access")
    expect(windowStub.sessionStorage.getItem(REFRESH_TOKEN_KEY)).toBe("legacy-refresh")
    expect(windowStub.localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(windowStub.localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
  })

  it("saveSession and clearSession keep storage in sync and emit change events", async () => {
    const windowStub = installWindow()
    const { clearSession, saveSession, subscribeAuthSessionChange } = await importAuthModule()
    const events: Array<{
      accessToken: string | null
      refreshToken: string | null
      reason: "login" | "refresh" | "logout"
    }> = []

    const unsubscribe = subscribeAuthSessionChange((detail) => {
      events.push(detail)
    })

    saveSession("access-1", "refresh-1")
    clearSession()
    unsubscribe()

    expect(windowStub.sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(windowStub.sessionStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    expect(events).toEqual([
      {
        accessToken: "access-1",
        refreshToken: "refresh-1",
        reason: "login",
      },
      {
        accessToken: null,
        refreshToken: null,
        reason: "logout",
      },
    ])
  })
})
