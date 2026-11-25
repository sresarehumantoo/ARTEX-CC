export function DemoBanner() {
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-center gap-x-3 gap-y-0.5 border-b border-amber-500/30 bg-amber-500/10 px-4 py-1 text-center text-xs text-amber-900 dark:text-amber-200">
      <span>Live demo: a fictional, seeded engagement. Nothing here is real.</span>
      <span className="flex gap-3">
        <a href="https://artex-cc.com" className="font-medium underline underline-offset-2 hover:no-underline">
          artex-cc.com
        </a>
        <a href="https://github.com/SREsAreHumanToo/ARTEX-CC" className="font-medium underline underline-offset-2 hover:no-underline">
          GitHub
        </a>
      </span>
    </div>
  )
}
