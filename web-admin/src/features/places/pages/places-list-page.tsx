import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Building2Icon, PlusIcon } from "lucide-react"

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
import { placePath } from "@/features/mistyislet-shell/navigation"
import { summarizePlaceCounts } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import { createPlace, type CurrentUser } from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

export function PlacesAdaptedPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [placeName, setPlaceName] = useState("")
  const [address, setAddress] = useState("")
  const [region, setRegion] = useState("ID-JK")
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const places = resourceQuery.summary.places
  const tenantID = getViewerTenantID(viewer)
  const canMutate = Boolean(tenantID && !resourceQuery.usingFallback)

  const createPlaceMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return createPlace(token, {
        tenant_id: tenantID,
        name: placeName.trim(),
        address: address.trim() || undefined,
        region: region.trim() || undefined,
      })
    },
    onSuccess: async () => {
      setCreateOpen(false)
      setPlaceName("")
      setAddress("")
      setRegion("ID-JK")
      setActionNotice("Place created.")
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Place create failed")
    },
  })

  return (
    <>
      <PageFrame
        breadcrumbs={[t("common.home"), "Places"]}
        title="Places"
        count={resourceQuery.isPending ? "--" : places.length}
        description="Open a place to switch into Place Admin navigation"
        actions={
          <Button
            disabled={!canMutate}
            onClick={() => {
              setActionNotice("")
              setActionError("")
              setCreateOpen(true)
            }}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Create Place
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            Live place resources are unavailable. Showing reference data.
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

        {places.length > 0 ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {places.map((place) => (
              <Link
                key={place.id}
                to={placePath("dashboard", place.id)}
                className="rounded-[6px] border border-[#d9dbe3] bg-white p-5 transition-colors hover:bg-[#fbfbfc]"
              >
                <Building2Icon className="size-6 text-[#6f717c]" />
                <h2 className="mt-5 text-lg font-semibold text-[#17171c]">{place.name}</h2>
                <p className="mt-2 text-sm text-[#6f717c]">{summarizePlaceCounts(place)}</p>
                <p className="mt-1 truncate text-sm text-[#9a9ca7]">{place.region}</p>
                <div className="mt-5">
                  <StatusDot tone={place.tone} label={place.statusLabel} />
                </div>
              </Link>
            ))}
          </div>
        ) : (
          <section className="rounded-[6px] border border-[#d9dbe3] bg-white px-6 py-12 text-center">
            <Building2Icon className="mx-auto size-8 text-[#9a9ca7]" />
            <p className="mt-3 text-sm font-semibold text-[#17171c]">{t("kisi.places.noPlaces")}</p>
            <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.places.emptyPrompt")}</p>
          </section>
        )}
      </PageFrame>

      <Sheet open={createOpen} onOpenChange={setCreateOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.places.createPlace")}</SheetTitle>
            <SheetDescription>{t("kisi.places.emptyPrompt")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createPlaceMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={placeName}
                onChange={(event) => setPlaceName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.description")}</span>
              <input
                value={address}
                onChange={(event) => setAddress(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.place")}</span>
              <input
                value={region}
                onChange={(event) => setRegion(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || createPlaceMutation.isPending || !placeName.trim()}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createPlaceMutation.isPending ? "Creating..." : "Create Place"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}
