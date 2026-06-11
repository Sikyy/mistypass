import { describe, expect, it } from "vitest"

import { filterExpectedGuests, ndaStepNeeded, type KioskGuest, type KioskNDATemplate } from "./kiosk-page-utils"

const guests: KioskGuest[] = [
  { id: "g1", name: "Andri Pratama", phone: "0812 1111", host_name: "Siti", company: "Acme", status: "expected" },
  { id: "g2", name: "Budi Santoso", phone: "0813 2222", host_name: "Joko", company: "Beta Corp", status: "expected" },
  { id: "g3", name: "Checked In", phone: "0814 3333", host_name: "Siti", status: "checked_in" },
  { id: "g4", name: "Cancelled Guy", phone: "0815 4444", host_name: "Joko", status: "cancelled" },
]

describe("filterExpectedGuests", () => {
  it("returns only expected guests, sorted by name", () => {
    const out = filterExpectedGuests(guests, "")
    expect(out.map((g) => g.id)).toEqual(["g1", "g2"])
  })

  it("matches name, phone, company and host name case-insensitively", () => {
    expect(filterExpectedGuests(guests, "andri").map((g) => g.id)).toEqual(["g1"])
    expect(filterExpectedGuests(guests, "0813").map((g) => g.id)).toEqual(["g2"])
    expect(filterExpectedGuests(guests, "beta").map((g) => g.id)).toEqual(["g2"])
    expect(filterExpectedGuests(guests, "siti").map((g) => g.id)).toEqual(["g1"])
  })

  it("returns empty when nothing matches", () => {
    expect(filterExpectedGuests(guests, "zzz")).toEqual([])
  })
})

describe("ndaStepNeeded", () => {
  const base: KioskNDATemplate = { title: "NDA", body: "Keep secrets.", version: 1, required: true }

  it("requires the NDA step when a template exists", () => {
    expect(ndaStepNeeded(base)).toBe(true)
    expect(ndaStepNeeded({ ...base, required: false })).toBe(true)
  })

  it("skips the NDA step when no template is configured (version 0)", () => {
    expect(ndaStepNeeded({ title: "", body: "", version: 0, required: false })).toBe(false)
    expect(ndaStepNeeded(undefined)).toBe(false)
  })
})
