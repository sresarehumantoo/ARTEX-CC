import { useMemo } from 'react'

const KEYWORDS = new Set(
  (
    'abstract as async await break case catch class const continue def default defer do elif else enum except export extends false ' +
    'finally fn for from func function go if impl import in interface let match new nil none null package pub return select self static struct ' +
    'switch this throw true try type typeof use var while with yield'
  ).split(' '),
)

const TOKEN =
  /(\/\/.*|#.*|\/\*[\s\S]*?\*\/)|("(?:\\.|[^"\\\n])*"|'(?:\\.|[^'\\\n])*'|`(?:\\.|[^`\\])*`)|(\b\d+(?:\.\d+)?\b)|([A-Za-z_]\w*)/g

function highlight(code: string): React.ReactNode[] {
  const out: React.ReactNode[] = []
  let last = 0
  let i = 0
  for (const m of code.matchAll(TOKEN)) {
    const idx = m.index ?? 0
    if (idx > last) out.push(code.slice(last, idx))
    last = idx + m[0].length
    const [text, comment, str, num, word] = m
    let cls = ''
    if (comment) cls = 'text-muted-foreground italic'
    else if (str) cls = 'text-emerald-700 dark:text-emerald-400'
    else if (num) cls = 'text-amber-700 dark:text-amber-400'
    else if (word && KEYWORDS.has(word)) cls = 'text-violet-700 dark:text-violet-400'
    out.push(cls ? <span key={i++} className={cls}>{text}</span> : text)
  }
  if (last < code.length) out.push(code.slice(last))
  return out
}

export function CodeBlock({ code, language }: { code: string; language?: string }) {
  const nodes = useMemo(() => highlight(code), [code])
  return (
    <div className="overflow-hidden rounded-lg bg-muted">
      {language && <div className="border-b border-border px-3 py-1 font-mono text-[10px] text-muted-foreground uppercase">{language}</div>}
      <pre className="overflow-x-auto p-3 font-mono text-xs">
        <code>{nodes}</code>
      </pre>
    </div>
  )
}
