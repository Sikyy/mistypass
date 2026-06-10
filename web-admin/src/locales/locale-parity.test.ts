import { describe, expect, it } from "vitest"

import enUS from "@/locales/en-US.json"
import idID from "@/locales/id-ID.json"
import zhCN from "@/locales/zh-CN.json"

type LocaleTree = { [key: string]: string | LocaleTree }

function flattenKeys(tree: LocaleTree, prefix = "", out: string[] = []): string[] {
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === "object") {
      flattenKeys(value, path, out)
    } else {
      out.push(path)
    }
  }
  return out
}

const referenceKeys = flattenKeys(zhCN as LocaleTree)

const targetLocales: Array<[name: string, tree: LocaleTree]> = [
  ["en-US", enUS as LocaleTree],
  ["id-ID", idID as LocaleTree],
]

describe("locale key parity", () => {
  it("zh-CN reference locale has keys", () => {
    expect(referenceKeys.length).toBeGreaterThan(0)
  })

  it.each(targetLocales)("%s contains every key present in zh-CN", (name, tree) => {
    const targetKeys = new Set(flattenKeys(tree))
    const missing = referenceKeys.filter((key) => !targetKeys.has(key))
    expect(
      missing,
      `${missing.length} key(s) present in zh-CN.json are missing from ${name}.json:\n  ${missing.join("\n  ")}`,
    ).toEqual([])
  })
})
