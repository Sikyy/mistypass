import { cn } from "@/lib/utils"

type MistyIslandMarkProps = {
  className?: string
  markClassName?: string
  title?: string
}

export function MistyIslandMark({ className, markClassName, title = "MistyPass" }: MistyIslandMarkProps) {
  return (
    <div className={cn("relative inline-flex items-center justify-center", className)} aria-label={title}>
      <div className="absolute inset-0 rounded-full bg-white/20 blur-xl" aria-hidden />
      <svg
        viewBox="0 0 96 64"
        role="img"
        aria-hidden={title ? undefined : true}
        className={cn("relative h-10 w-14 drop-shadow-[0_0_16px_rgba(255,255,255,0.38)]", markClassName)}
      >
        <title>{title}</title>
        <path
          d="M12 43c7-3.2 16.8-4.6 24.9-4.8 6.6-.2 8.5-13.1 14.7-21.7 2.6-3.7 6.9-3.9 9.1.4 2.9 5.8 2.7 13.1 7.9 15.4 3.4 1.5 8.1.2 12.7 3.7 3.1 2.4 5.6 4.9 9.6 5.2 2.5.2 4.2 1.8 3.5 3.8-.9 2.6-6.1 3.9-14.5 3.9H26.2c-8.9 0-14.8-2-14.2-5.9Z"
          className="fill-white/92"
        />
        <path
          d="M19 49c7.1 2.8 18.6 2 27.5 3.8 8.6 1.7 3.6 5.6 14.6 7.1 11.7 1.6 25.3-.8 28-4.2 2.3-2.8-4.1-5.1-14.7-5.1H31.2c-4.1 0-8.2-.6-12.2-1.6Z"
          className="fill-white/54"
        />
        <path
          d="M2 45c13.5 7.3 28.5 7 43.1 6.6 15.3-.4 31.1-1.2 47.9 3.1"
          className="fill-none stroke-white/45"
          strokeLinecap="round"
          strokeWidth="2"
        />
        <path
          d="M16 36c8.5 1.8 15.7.9 23.5-.5 10.1-1.8 21.2-3 36.5 2.5"
          className="fill-none stroke-white/28"
          strokeLinecap="round"
          strokeWidth="1.5"
        />
        <ellipse cx="49" cy="48" rx="34" ry="5.4" className="fill-white/12" />
      </svg>
    </div>
  )
}
