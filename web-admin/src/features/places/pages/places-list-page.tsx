import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Building2Icon, PlusIcon, StarIcon } from "lucide-react"

import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
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
import { createPlace, favoritePlace, unfavoritePlace, type CurrentUser } from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

export function PlacesAdaptedPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [placeName, setPlaceName] = useState("")
  const [address, setAddress] = useState("")
  const [region, setRegion] = useState("ID-JK")
  const [actionNotice, setActionNotice] = useState("")
  const [favoritePlaces, setFavoritePlaces] = useState<Set<string>>(new Set())
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
      setActionNotice(t("kisi.places.created"))
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
        title={t("kisi.places.title")}
        count={resourceQuery.isPending ? "--" : places.length}
        description={t("kisi.places.description")}
        actions={
          <Button
            disabled={!canMutate}
            onClick={() => {
              setActionNotice("")
              setActionError("")
              setCreateOpen(true)
            }}
            className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Create Place
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="mp-alert-warning">
            Live place resources are unavailable. Showing reference data.
          </div>
        ) : null}
        {actionNotice ? (
          <div className="mp-alert-success">
            {actionNotice}
          </div>
        ) : null}
        {actionError ? (
          <div className="mp-alert-warning">
            {actionError}
          </div>
        ) : null}

        {places.length > 0 ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {places.map((place) => (
              <Link
                key={place.id}
                to={placePath("dashboard", place.id)}
                className="rounded-[6px] border border-line-default bg-white p-5 transition-colors hover:bg-surface-page"
              >
                <div className="flex items-start justify-between">
                  <Building2Icon className="size-6 text-content-subtle" />
                  <button
                    type="button"
                    className="flex size-8 items-center justify-center rounded-[6px] hover:bg-surface-sunken"
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      const isFav = favoritePlaces.has(place.id)
                      if (isFav) {
                        unfavoritePlace(token, place.id)
                        setFavoritePlaces((prev) => { const next = new Set(prev); next.delete(place.id); return next })
                      } else {
                        favoritePlace(token, place.id)
                        setFavoritePlaces((prev) => new Set(prev).add(place.id))
                      }
                    }}
                    aria-label={favoritePlaces.has(place.id) ? "Unfavorite" : "Favorite"}
                  >
                    <StarIcon className={`size-4 ${favoritePlaces.has(place.id) ? "fill-amber-400 text-amber-400" : "text-content-muted"}`} />
                  </button>
                </div>
                <h2 className="mt-3 text-lg font-semibold text-content-heading">{place.name}</h2>
                <p className="mt-2 text-sm text-content-subtle">{summarizePlaceCounts(place)}</p>
                <p className="mt-1 truncate text-sm text-content-muted">{place.region}</p>
                <div className="mt-5">
                  <StatusDot tone={place.tone} label={place.statusLabel} />
                </div>
              </Link>
            ))}
          </div>
        ) : (
          <section className="rounded-[6px] border border-line-default bg-white px-6 py-12 text-center">
            <Building2Icon className="mx-auto size-8 text-content-muted" />
            <p className="mt-3 text-sm font-semibold text-content-heading">{t("kisi.places.noPlaces")}</p>
            <p className="mt-1 text-sm text-content-subtle">{t("kisi.places.emptyPrompt")}</p>
          </section>
        )}
      </PageFrame>

      <Sheet open={createOpen} onOpenChange={setCreateOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[460px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={placeName}
                onChange={(event) => setPlaceName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.description")}</span>
              <input
                value={address}
                onChange={(event) => setAddress(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.place")}</span>
              <input
                value={region}
                onChange={(event) => setRegion(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            {actionError ? (
              <div className={cn("mp-alert-warning", "px-4 py-3")}>
                {actionError}
              </div>
            ) : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)} className="h-10 rounded-[6px]">
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!canMutate || createPlaceMutation.isPending || !placeName.trim()}
                className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover disabled:bg-[#c6c8d2]"
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
