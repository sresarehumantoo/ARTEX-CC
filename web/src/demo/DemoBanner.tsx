export function DemoBanner() {
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-center gap-x-4 gap-y-1 border-b border-amber-500/30 bg-amber-500/15 px-4 py-2.5 text-center text-sm text-amber-900 dark:text-amber-200">
      <span className="font-medium">Live demo: a fictional, seeded engagement.</span>
      <span className="flex gap-4">
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
