type CreateTerminalInput = {
  server_id: string
  title?: string
  task_id?: string
}

type CreateTerminalFn = (payload: CreateTerminalInput) => Promise<{ id: string } | null | undefined>

export type EnsureWorkbenchTerminalOptions = {
  taskId: string
  title?: string
  automationMode?: string | null
  serverId?: string | null
  targetServerIds?: string[] | null
  createTerminal: CreateTerminalFn
}

function trimText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

export function resolvePreferredServerId(input: {
  automationMode?: string | null
  serverId?: string | null
  targetServerIds?: string[] | null
}) {
  const mode = trimText(input.automationMode).toLowerCase()
  const serverId = trimText(input.serverId)
  const firstTarget = Array.isArray(input.targetServerIds)
    ? trimText(input.targetServerIds.find((id) => trimText(id) !== ''))
    : ''

  if (mode === 'cli') return serverId || firstTarget
  if (mode === 'script' || mode === 'agent') return firstTarget || serverId
  return serverId || firstTarget
}

export async function ensureWorkbenchTerminal(options: EnsureWorkbenchTerminalOptions) {
  const taskId = trimText(options.taskId)
  if (!taskId) return ''

  const serverId = resolvePreferredServerId({
    automationMode: options.automationMode,
    serverId: options.serverId,
    targetServerIds: options.targetServerIds
  })
  if (!serverId) return ''

  const created = await options.createTerminal({
    server_id: serverId,
    task_id: taskId,
    title: trimText(options.title) || undefined
  })

  return trimText(created?.id)
}
