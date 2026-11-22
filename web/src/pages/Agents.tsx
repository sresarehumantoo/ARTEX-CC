import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Bot, Plus } from 'lucide-react'
import { getAgents, type Agent } from '../lib/api/agents'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Skeleton } from '../components/ui/skeleton'
import AgentDetail from './AgentDetail'

export default function Agents() {
  const { data, isLoading, error } = useQuery({ queryKey: ['agents'], queryFn: getAgents })
  const [editing, setEditing] = useState<Agent | null>(null)
  const [open, setOpen] = useState(false)

  const openAgent = (agent: Agent | null) => {
    setEditing(agent)
    setOpen(true)
  }

  return (
    <div className="p-4 md:p-6 space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">Agents</h1>
          <p className="text-sm text-muted-foreground">Presets for chat conversations.</p>
        </div>
        <Button onClick={() => openAgent(null)}>
          <Plus /> New Agent
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">Failed to load agents: {error.message}</p>}

      {isLoading ? (
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map(i => <Skeleton key={i} className="h-32 rounded-xl" />)}
        </div>
      ) : data && data.length === 0 ? (
        <p className="text-sm text-muted-foreground italic">No agents yet.</p>
      ) : (
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {data?.map(agent => (
            <button
              key={agent.id}
              type="button"
              onClick={() => openAgent(agent)}
              className="text-left rounded-xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <Card className="h-full transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Bot className="size-4 text-muted-foreground" />
                    <span className="truncate">{agent.name}</span>
                  </CardTitle>
                  <CardDescription className="line-clamp-2">
                    {agent.description || 'No description'}
                  </CardDescription>
                </CardHeader>
                <CardContent className="mt-auto flex flex-wrap gap-2">
                  {agent.model && <Badge variant="secondary">{agent.model}</Badge>}
                  <Badge variant="outline">
                    {agent.allowed_tools.length} {agent.allowed_tools.length === 1 ? 'tool' : 'tools'}
                  </Badge>
                </CardContent>
              </Card>
            </button>
          ))}
        </div>
      )}

      <AgentDetail open={open} agent={editing} onOpenChange={setOpen} />
    </div>
  )
}
