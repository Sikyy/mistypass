import { Link } from "react-router-dom"
import { Building2Icon, PlusIcon } from "lucide-react"

import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import { placePath } from "@/features/kisi-shell/navigation"
import { summarizePlaceCounts } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"

export function PlacesAdaptedPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const places = resourceQuery.summary.places

  return (
    <PageFrame
      breadcrumbs={["Home", "Places"]}
      title="Places"
      count={resourceQuery.isPending ? "--" : places.length}
      description="Open a place to switch into Place Admin navigation"
      actions={
        <Button className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]">
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
          <p className="mt-3 text-sm font-semibold text-[#17171c]">No places found</p>
          <p className="mt-1 text-sm text-[#6f717c]">Create a place before assigning doors and hardware.</p>
        </section>
      )}
    </PageFrame>
  )
}
