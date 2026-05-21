import { useEffect, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation } from "react-router"
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
  formatMistyisletRoleLabel,
  isNavEntryActive,
  resolveNavSections,
  type NavEntry,
} from "./navigation"

type MistyisletAdminShellProps = {
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
        className="relative flex h-10 items-center gap-3 rounded-[6px] bg-brand px-3 text-sm font-semibold text-white"
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
        className="flex h-10 w-full items-center gap-3 rounded-[6px] px-3 text-left text-sm font-medium text-white/68 transition-colors hover:bg-white/8 hover:text-white/90"
      >
        <Icon className="size-4 text-white/38" />
        <span>{entry.label}</span>
      </Link>
    )
  }

  return (
    <button
      type="button"
      disabled
      className="flex h-10 w-full items-center gap-3 rounded-[6px] px-3 text-left text-sm font-medium text-white/68 transition-colors hover:bg-white/8 disabled:cursor-default"
    >
      <Icon className="size-4 text-white/38" />
      <span>{entry.label}</span>
    </button>
  )
}

const LANGUAGE_OPTIONS = [
  { code: "en-US", label: "English" },
  { code: "zh-CN", label: "中文" },
  { code: "id-ID", label: "Bahasa" },
] as const

function GlobalTopBar({ viewer, onLogout }: Omit<MistyisletAdminShellProps, "children">) {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const { currentView, selectedPlaceName } = useNavigationContext()
  const accountTitle =
    currentView === "place" && selectedPlaceName
      ? t("kisi.shell.placeAdmin", { place: selectedPlaceName })
      : t("kisi.shell.orgAdmin")
  const roleLabel = formatMistyisletRoleLabel(viewer, location.pathname, t)

  return (
    <header className="sticky top-0 z-40 hidden h-[64px] items-center bg-[#0d0d0c] text-white shadow-[0_1px_0_rgba(0,0,0,0.18)] lg:flex">
      <div className="flex h-full w-[248px] shrink-0 items-center px-8">
        <Link to="/home" className="text-[27px] font-bold leading-none tracking-[0.02em]">
          Mistyislet
        </Link>
      </div>

      <div className="hidden h-11 min-w-0 w-[520px] items-center gap-3 rounded-[6px] bg-white px-3 text-content-body shadow-[0_0_0_1px_rgba(255,255,255,0.16)] md:flex">
        <SearchIcon className="size-5 shrink-0 text-content-subtle" />
        <input
          type="text"
          placeholder={t("kisi.shell.searchPlaceholder")}
          className="min-w-0 flex-1 truncate bg-transparent text-sm text-content-body placeholder:text-content-subtle outline-none"
        />
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
          <span className="absolute right-1.5 top-1.5 flex size-4 items-center justify-center rounded-full bg-[#fff45c] text-[10px] font-bold text-[#0d0d0c]">
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
            className="w-[276px] rounded-[4px] border-line-default bg-white p-0 text-content-body shadow-[0_8px_22px_rgba(23,23,28,0.14)]"
          >
            <DropdownMenuLabel className="px-4 py-3 text-xs font-semibold text-content-subtle">
              {roleLabel}
            </DropdownMenuLabel>
            <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading focus:bg-surface-page focus:text-content-heading">
              <Link to="/my-account">{t("kisi.shell.myAccount")}</Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading focus:bg-surface-page focus:text-content-heading">
              <a href="https://docs.mistyislet.com/help" target="_blank" rel="noopener noreferrer">{t("kisi.shell.helpSupport")}</a>
            </DropdownMenuItem>
            <DropdownMenuSeparator className="m-0 bg-line-subtle" />
            <DropdownMenuLabel className="px-4 py-2 text-xs font-semibold text-content-subtle">
              {t("kisi.shell.language", { lang: LANGUAGE_OPTIONS.find(l => (i18n.resolvedLanguage ?? i18n.language).startsWith(l.code.split("-")[0]))?.label ?? "English" })}
            </DropdownMenuLabel>
            {LANGUAGE_OPTIONS.map((lang) => (
              <DropdownMenuItem
                key={lang.code}
                className="cursor-pointer rounded-none px-4 py-2.5 text-sm text-content-heading focus:bg-surface-page focus:text-content-heading"
                onSelect={() => void i18n.changeLanguage(lang.code)}
              >
                <span className="min-w-0 flex-1">{lang.label}</span>
                {(i18n.resolvedLanguage ?? i18n.language).startsWith(lang.code.split("-")[0]) ? (
                  <span className="size-2 rounded-full bg-brand" />
                ) : null}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator className="m-0 bg-line-subtle" />
            <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-content-heading focus:bg-surface-page focus:text-content-heading">
              {t("kisi.shell.addAccount")}
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading focus:bg-surface-page focus:text-content-heading" onSelect={onLogout}>
              {t("kisi.shell.signOut")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}

function AdminSidebar({ viewer }: Pick<MistyisletAdminShellProps, "viewer">) {
  const { t } = useTranslation()
  const location = useLocation()
  const { currentView, selectedPlaceName, backToOrganization } = useNavigationContext()
  const sections = resolveNavSections(viewer, location.pathname, t)
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
    <aside className="sticky top-[64px] hidden h-[calc(100vh-64px)] w-[248px] shrink-0 flex-col overflow-hidden border-r border-white/10 bg-[#0d0d0c] lg:flex">
      <nav className="flex-1 overflow-auto px-4 py-9">
        {currentView === "place" && selectedPlaceName ? (
          <div className="mb-5 rounded-[6px] border border-white/10 bg-white/5 px-4 py-3">
            <p className="text-xs font-semibold text-white/42">{t("kisi.shell.place")}</p>
            <p className="mt-1 truncate text-sm font-semibold text-white/90">{selectedPlaceName}</p>
            {canReturnToOrganization ? (
              <button
                type="button"
                onClick={backToOrganization}
                className="mt-3 text-left text-xs font-semibold text-brand hover:underline"
              >
                {t("kisi.shell.backToOrg")}
              </button>
            ) : null}
          </div>
        ) : null}
        {sections.map((section, index) => (
          <div key={section.title ?? `section-${index}`} className={cn("space-y-1", index > 0 && "mt-3 border-t border-white/10 pt-3")}>
            {section.collapsible ? (
              <button
                type="button"
                onClick={() => setOrganizationSetupOpen((current) => !current)}
                aria-expanded={organizationSetupOpen}
                className="mb-2 flex h-8 w-full items-center gap-2 rounded-[6px] px-2 text-left text-sm font-medium text-white/68 hover:bg-white/8"
              >
                <ChevronDownIcon className={cn("size-4 text-white/52 transition-transform", !organizationSetupOpen && "-rotate-90")} />
                <SettingsIcon className="size-4 text-white/38" />
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

      <div className="space-y-2 border-t border-white/10 px-4 py-3">
        <div className="space-y-1">
          <a
            href="https://shop.mistyislet.com"
            target="_blank"
            rel="noopener noreferrer"
            className="flex h-9 w-full min-w-0 items-center gap-3 rounded-[6px] px-3 text-left text-sm font-medium text-white/58 hover:bg-white/8 hover:text-white/80"
          >
            <ShoppingBagIcon className="size-4 shrink-0 text-white/38" />
            <span className="min-w-0 flex-1 truncate whitespace-nowrap">{t("kisi.shell.shop")}</span>
          </a>
          <a
            href="https://docs.mistyislet.com"
            target="_blank"
            rel="noopener noreferrer"
            className="flex h-9 w-full min-w-0 items-center gap-3 rounded-[6px] px-3 text-left text-sm font-medium text-white/58 hover:bg-white/8 hover:text-white/80"
          >
            <BookOpenIcon className="size-4 shrink-0 text-white/38" />
            <span className="min-w-0 flex-1 truncate whitespace-nowrap">{t("kisi.shell.documentation")}</span>
          </a>
        </div>
        <a
          href="https://docs.mistyislet.com/help"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-1 flex h-10 w-full min-w-0 items-center gap-3 rounded-[6px] border-t border-white/10 px-3 pt-2 text-left text-sm font-medium text-white/58 hover:bg-white/8 hover:text-white/80"
        >
          <CircleHelpIcon className="size-4 shrink-0 text-white/38" />
          <span className="min-w-0 flex-1 truncate whitespace-nowrap">{t("kisi.shell.helpFeedback")}</span>
        </a>
      </div>
    </aside>
  )
}

function MobileTopBar({ viewer, onLogout }: Omit<MistyisletAdminShellProps, "children">) {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const roleLabel = formatMistyisletRoleLabel(viewer, location.pathname, t)

  return (
    <header className="sticky top-0 z-40 bg-[#0d0d0c] text-white lg:hidden">
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
            <DropdownMenuContent align="end" className="w-64 rounded-[4px] border-line-default bg-white p-0 text-content-body">
              <DropdownMenuLabel className="px-4 py-3 text-xs font-semibold text-content-subtle">{roleLabel}</DropdownMenuLabel>
              <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading">
                <Link to="/my-account">{t("kisi.shell.myAccount")}</Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading">
                <a href="https://docs.mistyislet.com/help" target="_blank" rel="noopener noreferrer">{t("kisi.shell.helpSupport")}</a>
              </DropdownMenuItem>
              <DropdownMenuSeparator className="m-0 bg-line-subtle" />
              {LANGUAGE_OPTIONS.map((lang) => (
                <DropdownMenuItem
                  key={lang.code}
                  className="cursor-pointer rounded-none px-4 py-2.5 text-sm text-content-heading"
                  onSelect={() => void i18n.changeLanguage(lang.code)}
                >
                  <span className="min-w-0 flex-1">{lang.label}</span>
                  {(i18n.resolvedLanguage ?? i18n.language).startsWith(lang.code.split("-")[0]) ? (
                    <span className="size-2 rounded-full bg-brand" />
                  ) : null}
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator className="m-0 bg-line-subtle" />
              <DropdownMenuItem className="cursor-default rounded-none px-4 py-3 text-sm text-content-heading">
                {t("kisi.shell.addAccount")}
              </DropdownMenuItem>
              <DropdownMenuItem className="cursor-pointer rounded-none px-4 py-3 text-sm text-content-heading" onSelect={onLogout}>
                {t("kisi.shell.signOut")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      <div className="border-t border-white/10 px-5 pb-4">
        <div className="flex h-10 items-center gap-2 rounded-[6px] bg-white px-3 text-content-body">
          <SearchIcon className="size-4 text-content-subtle" />
          <input
            type="text"
            placeholder={t("kisi.shell.searchMobile")}
            className="min-w-0 flex-1 truncate bg-transparent text-sm text-content-body placeholder:text-content-subtle outline-none"
          />
        </div>
        <p className="mt-2 truncate text-xs text-white/60">{roleLabel}</p>
      </div>
    </header>
  )
}

export function MistyisletAdminShell({ viewer, onLogout, children }: MistyisletAdminShellProps) {
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
      <div className="min-h-screen bg-[#f7f7f8] text-content-heading">
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
