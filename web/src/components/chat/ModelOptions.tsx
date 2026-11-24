import { groupModels } from '../../lib/api/llm'
import type { ModelOption } from '../../lib/api/llm'
import { SelectGroup, SelectItem, SelectLabel } from '../ui/select'

export function ModelOptions({ models }: { models: ModelOption[] }) {
  return (
    <>
      {groupModels(models).map(([group, items]) => (
        <SelectGroup key={group}>
          <SelectLabel>{group}</SelectLabel>
          {items.map(m => (
            <SelectItem key={m.id} value={m.id}>
              {m.label}
            </SelectItem>
          ))}
        </SelectGroup>
      ))}
    </>
  )
}
