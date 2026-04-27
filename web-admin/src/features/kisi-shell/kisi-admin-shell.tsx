import { useEffect, useState, type ReactNode } from "react"
import { Link, useLocation } from "react-router-dom"
import {
  BellIcon,
  BookOpenIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleHelpIcon,
  RocketIcon,
  SearchIcon,
  SettingsIcon,
  ShoppingBagIcon,
} from "lucide-react"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { NavigationProvider, useNavigationContext } from "@/context/navigation-context"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

import {
  formatKisiRoleLabel,
  isNavEntryActive,
  resolveNavSections,
  type NavEntry,
} from "./navigation"

type KisiAdminShellProps = {
  viewer: CurrentUser
  onLogout: () => void
  children: ReactNode
}

function NavItem({ entry, pathname }: { entry: NavEntry; pathname: string }) {
  const Icon = entry.icon
  const active = isNavEntryActive(entry, pathname)

  if (active && entry.to) {
    return (
      <Link
        to={entry.to ?? "/home"}
        className="relative flex h-10 items-center gap-3 rounded-[6px] bg-[#4f55ff] px-3 text-sm font-semibold text-white"
      >
        <span className="absolute left-0 top-2 h-6 w-[3px] rounded-r-full bg-white/95" />
        <Icon className="size-4" />
        <span>{entry.label}</span>
      </Link>
    )
  }

  if (entry.to) {
    return (
      <Link
        to={entry.to}
        className="flex h-10 w-full items-center gap-3 rounded-[6px] px-3 text-left text-sm font-semibold text-[#2f3037] transition-colors hover:bg-[#f7f7f8]"
      >
        <Icon className="size-4 text-[#6f717c]" />
        <span>{entry.label}</span>
      </Link>
    )
  }

  return (
    <button
      type="button"
      disabled
      className="flex h-10 w-full items-center gap-3 rounded-[6px] px-3 text-left text-sm font-semibold text-[#2f3037] transition-colors hover:bg-[#f7f7f8] disabled:cursor-default"
    >
      <Icon className="size-4 text-[#6f717c]" />
      <span>{entry.label}</span>
    </button>
  )
}

function GlobalTopBar({ viewer, onLogout }: Omit<KisiAdminShellProps, "children">) {
  const location = useLocation()
  const { currentView, selectedPlaceName } = useNavigationContext()
  const accountTitle =
    currentView === "place" && selectedPlaceName
      ? `${selectedPlaceName} / Place Admin`
      : "Mistyislet / Organization Admin"
  const roleLabel = formatKisiRoleLabel(viewer, location.pathname)

  return (
    <header className="sticky top-0 z-40 hidden h-[64px] items-center bg-[#202443] text-white shadow-[0_1px_0_rgba(0,0,0,0.18)] lg:flex">
      <div className="flex h-full w-[248px] shrink-0 items-center px-8">
        <Link to="/home" className="text-[27px] font-bold leading-none tracking-[0.02em]">
          Mistyislet
        </Link>
      </div>

      <div className="hidden h-11 min-w-0 w-[520px] items-center gap-3 rounded-[6px] bg-white px-3 text-[#2f3037] shadow-[0_0_0_1px_rgba(255,255,255,0.16)] md:flex">
        <SearchIcon className="size-5 shrink-0 text-[#6f717c]" />
        <span className="truncate text-sm text-[#2f3037]">
          Search docs, users, doors, places, and features
        </span>
        <ChevronDownIcon className="ml-auto size-4 shrink-0 text-[#2f3037]" />
      </div>

      <div className="ml-auto flex items-center gap-3 px-8">
        <button
          type="button"
          className="hidden size-9 items-center justify-center rounded-full text-white/82 transition-colors hover:bg-white/10 md:flex"
          aria-label="Product launchpad"
        >
          <RocketIcon className="size-5" />
        </button>
        <button
          type="button"
          className="relative flex size-9 items-center justify-center rounded-full text-white/82 transition-colors hover:bg-white/10"
          aria-label="Notifications"
        >
          <BellIcon className="size-5" />
          <span className="absolute right-1.5 top-1.5 flex size-4 items-center justify-center rounded-full bg-[#fff45c] text-[10px] font-bold text-[#202443]">
            1
          </span>
        </button>
        <div className="flex size-7 items-center justify-center rounded-full border border-[#4cae5a]/70" aria-label="System online">
          <span className="size-4 rounded-full bg-[#35a853]" />
        </div>
        <div className="hidden h-8 w-px bg-white/45 md:block" />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="hidden min-w-[244px] max-w-[330px] items-center gap-2 rounded-[6px] px-3 py-1.5 text-left transition-colors hover:bg-white/10 sm:flex"
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-white">{accountTitle}</span>
                <span className="block truncate text-xs text-white/58">{viewer.email}</span>
              </span>
              <ChevronDownIcon className="size-4 shrink-0 text-white/68" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="w-[276px] rounded-[4px] border-[#d9dbe3] bg-white p-0 text-[#2f3037] shadow-[0_8px_22px_rgba(23,23,28,0.14)]"
          >
            <DropdownMenuLabel className="px-4 py-3 text-xs font-semibold text-[#6f717c]">
              {roleLabel}
            </DropdownMenuLabel>
            <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-[#17171c] focus:bg-[#f7f7f8] focus:text-[#17171c]">
              <Link to="/my-account">My Account</Link>
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c] focus:bg-[#f7f7f8] focus:text-[#17171c]">
              Help & Support
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c] focus:bg-[#f7f7f8] focus:text-[#17171c]">
              <span className="min-w-0 flex-1">Language: English</span>
              <ChevronRightIcon className="size-4 text-[#2f3037]" />
            </DropdownMenuItem>
            <DropdownMenuSeparator className="m-0 bg-[#eceef2]" />
            <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c] focus:bg-[#f7f7f8] focus:text-[#17171c]">
              Add Account
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c] focus:bg-[#f7f7f8] focus:text-[#17171c]" onSelect={onLogout}>
              Sign Out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}

function AdminSidebar({ viewer }: Pick<KisiAdminShellProps, "viewer">) {
  const location = useLocation()
  const { currentView, selectedPlaceName, backToOrganization } = useNavigationContext()
  const sections = resolveNavSections(viewer, location.pathname)
  const canReturnToOrganization = viewer.role === "super_admin" || viewer.role === "tenant_admin"
  const hasActiveCollapsibleSection = sections.some(
    (section) => section.collapsible && section.entries.some((entry) => isNavEntryActive(entry, location.pathname))
  )
  const [organizationSetupOpen, setOrganizationSetupOpen] = useState(hasActiveCollapsibleSection)

  useEffect(() => {
    if (hasActiveCollapsibleSection) {
      setOrganizationSetupOpen(true)
    }
  }, [hasActiveCollapsibleSection])

  return (
    <aside className="sticky top-[64px] hidden h-[calc(100vh-64px)] w-[248px] shrink-0 flex-col overflow-hidden border-r border-[#e1e3e8] bg-white lg:flex">
      <nav className="flex-1 overflow-auto px-4 py-9">
        {currentView === "place" && selectedPlaceName ? (
          <div className="mb-5 rounded-[6px] border border-[#e1e3e8] bg-[#fbfbfc] px-4 py-3">
            <p className="text-xs font-semibold text-[#6f717c]">Place</p>
            <p className="mt-1 truncate text-sm font-semibold text-[#17171c]">{selectedPlaceName}</p>
            {canReturnToOrganization ? (
              <button
                type="button"
                onClick={backToOrganization}
                className="mt-3 text-left text-xs font-semibold text-[#4f55ff] hover:underline"
              >
                ← Back to Organization
              </button>
            ) : null}
          </div>
        ) : null}
        {sections.map((section, index) => (
          <div key={section.title ?? `section-${index}`} className={cn("space-y-1", index > 0 && "mt-3 border-t border-[#d9dbe3] pt-3")}>
            {section.collapsible ? (
              <button
                type="button"
                onClick={() => setOrganizationSetupOpen((current) => !current)}
                aria-expanded={organizationSetupOpen}
                className="mb-2 flex h-8 w-full items-center gap-2 rounded-[6px] px-2 text-left text-sm font-semibold text-[#2f3037] hover:bg-[#f7f7f8]"
              >
                <ChevronDownIcon className={cn("size-4 text-[#2f3037] transition-transform", !organizationSetupOpen && "-rotate-90")} />
                <SettingsIcon className="size-4 text-[#6f717c]" />
                <span>{section.title}</span>
              </button>
            ) : null}
            {!section.collapsible && section.title && section.entries.length > 1 ? (
              <p className="sr-only">{section.title}</p>
            ) : null}
            {!section.collapsible || organizationSetupOpen
              ? section.entries.map((entry) => (
                  <NavItem key={entry.label} entry={entry} pathname={location.pathname} />
                ))
              : null}
          </div>
        ))}
      </nav>

      <div className="space-y-2 border-t border-[#d9dbe3] px-4 py-3">
        <div className="space-y-1">
          <Link
            to="/organization/create-place"
            className="flex h-9 w-full min-w-0 items-center gap-3 rounded-[6px] px-3 text-left text-sm font-semibold text-[#2f3037] hover:bg-[#f7f7f8]"
          >
            <ShoppingBagIcon className="size-4 shrink-0 text-[#6f717c]" />
            <span className="min-w-0 flex-1 truncate whitespace-nowrap">Mistyislet Shop</span>
          </Link>
          <Link
            to="/organization/settings"
            className="flex h-9 w-full min-w-0 items-center gap-3 rounded-[6px] px-3 text-left text-sm font-semibold text-[#2f3037] hover:bg-[#f7f7f8]"
          >
            <BookOpenIcon className="size-4 shrink-0 text-[#6f717c]" />
            <span className="min-w-0 flex-1 truncate whitespace-nowrap">Mistyislet Documentation</span>
          </Link>
        </div>
        <Link
          to="/organization/settings"
          className="mt-1 flex h-10 w-full min-w-0 items-center gap-3 rounded-[6px] border-t border-[#d9dbe3] px-3 pt-2 text-left text-sm font-semibold text-[#2f3037] hover:bg-[#f7f7f8]"
        >
          <CircleHelpIcon className="size-4 shrink-0 text-[#6f717c]" />
          <span className="min-w-0 flex-1 truncate whitespace-nowrap">Help & Feedback</span>
        </Link>
      </div>
    </aside>
  )
}

function MobileTopBar({ viewer, onLogout }: Omit<KisiAdminShellProps, "children">) {
  const location = useLocation()
  const roleLabel = formatKisiRoleLabel(viewer, location.pathname)

  return (
    <header className="sticky top-0 z-40 bg-[#202443] text-white lg:hidden">
      <div className="flex h-[64px] items-center justify-between px-5">
        <Link to="/home" className="text-2xl font-bold leading-none">
          Mistyislet
        </Link>
        <div className="flex items-center gap-3">
          <BellIcon className="size-5 text-white/82" />
          <span className="size-4 rounded-full bg-[#35a853]" />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex size-9 items-center justify-center rounded-full bg-white/10 text-xs font-bold text-white"
                aria-label="Open account menu"
              >
                {viewer.email.slice(0, 2).toUpperCase()}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64 rounded-[4px] border-[#d9dbe3] bg-white p-0 text-[#2f3037]">
              <DropdownMenuLabel className="px-4 py-3 text-xs font-semibold text-[#6f717c]">{roleLabel}</DropdownMenuLabel>
              <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-[#17171c]">
                <Link to="/my-account">My Account</Link>
              </DropdownMenuItem>
              <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c]">
                Help & Support
              </DropdownMenuItem>
              <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c]">
                <span className="min-w-0 flex-1">Language: English</span>
                <ChevronRightIcon className="size-4" />
              </DropdownMenuItem>
              <DropdownMenuSeparator className="m-0 bg-[#eceef2]" />
              <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c]">
                Add Account
              </DropdownMenuItem>
              <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-[#17171c]" onSelect={onLogout}>
                Sign Out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      <div className="border-t border-white/10 px-5 pb-4">
        <div className="flex h-10 items-center gap-2 rounded-[6px] bg-white px-3 text-[#2f3037]">
          <SearchIcon className="size-4 text-[#6f717c]" />
          <span className="truncate text-sm">Search docs and features</span>
        </div>
        <p className="mt-2 truncate text-xs text-white/60">{roleLabel}</p>
      </div>
    </header>
  )
}

export function KisiAdminShell({ viewer, onLogout, children }: KisiAdminShellProps) {
  useEffect(() => {
    const previousBodyBackground = document.body.style.background
    const previousBodyBackgroundImage = document.body.style.backgroundImage
    const previousColorScheme = document.documentElement.style.colorScheme

    document.body.style.background = "#f7f7f8"
    document.body.style.backgroundImage = "none"
    document.documentElement.style.colorScheme = "light"

    return () => {
      document.body.style.background = previousBodyBackground
      document.body.style.backgroundImage = previousBodyBackgroundImage
      document.documentElement.style.colorScheme = previousColorScheme
    }
  }, [])

  return (
    <NavigationProvider viewer={viewer}>
      <div className="min-h-screen bg-[#f7f7f8] text-[#17171c]">
        <GlobalTopBar viewer={viewer} onLogout={onLogout} />
        <MobileTopBar viewer={viewer} onLogout={onLogout} />
        <div className="lg:grid lg:min-h-[calc(100vh-64px)] lg:grid-cols-[248px_1fr]">
          <AdminSidebar viewer={viewer} />

          <main className="min-w-0">
            <div className="mx-auto max-w-[1180px] px-5 py-7 sm:px-8 lg:px-12 lg:py-10">
              {children}
            </div>
          </main>
        </div>
      </div>
    </NavigationProvider>
  )
}
