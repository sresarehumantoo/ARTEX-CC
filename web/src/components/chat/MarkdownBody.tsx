import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../ui/table'
import { cn } from '../../lib/utils'
import { CodeBlock } from './CodeBlock'

export function MarkdownBody({ text, className }: { text: string; className?: string }) {
  return (
    <div className={cn('min-w-0 space-y-2 text-sm leading-relaxed break-words', className)}>
      <Markdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ node: _node, ...props }) => (
            <a {...props} target="_blank" rel="noopener noreferrer" className="text-primary underline underline-offset-2" />
          ),
          p: ({ node: _node, ...props }) => <p {...props} className="whitespace-pre-wrap" />,
          ul: ({ node: _node, ...props }) => <ul {...props} className="list-disc space-y-1 pl-5" />,
          ol: ({ node: _node, ...props }) => <ol {...props} className="list-decimal space-y-1 pl-5" />,
          h1: ({ node: _node, ...props }) => <h1 {...props} className="text-base font-semibold" />,
          h2: ({ node: _node, ...props }) => <h2 {...props} className="text-sm font-semibold" />,
          h3: ({ node: _node, ...props }) => <h3 {...props} className="text-sm font-medium" />,
          blockquote: ({ node: _node, ...props }) => (
            <blockquote {...props} className="border-l-2 border-border pl-3 text-muted-foreground" />
          ),
          pre: ({ children }) => <>{children}</>,
          code: ({ node: _node, className: cls, children, ...props }) => {
            const lang = /language-(\w+)/.exec(cls ?? '')?.[1]
            const text = String(children)
            if (!lang && !text.includes('\n')) {
              return (
                <code {...props} className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                  {children}
                </code>
              )
            }
            return <CodeBlock code={text.replace(/\n$/, '')} language={lang} />
          },
          table: ({ node: _node, ...props }) => <Table {...props} />,
          thead: ({ node: _node, ...props }) => <TableHeader {...props} />,
          tbody: ({ node: _node, ...props }) => <TableBody {...props} />,
          tr: ({ node: _node, ...props }) => <TableRow {...props} />,
          th: ({ node: _node, ...props }) => <TableHead {...props} />,
          td: ({ node: _node, ...props }) => <TableCell {...props} />,
        }}
      >
        {text}
      </Markdown>
    </div>
  )
}
