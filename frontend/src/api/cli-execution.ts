import api from './index'
import type {
  CLIExecutionEvent,
  CLIExecutionSSEEnvelope,
  CLIExecutionSSEEventType,
  GetCLIExecutionResponse,
  ListCLIExecutionChildrenResponse,
  ListCLIExecutionEventsParams,
  ListCLIExecutionEventsResponse,
  ListCLIExecutionsParams,
  ListCLIExecutionsResponse,
  ResumeCLIExecutionRequest,
  ResumeCLIExecutionResponse
} from './types'

export interface StreamCLIExecutionEventsParams extends ListCLIExecutionEventsParams {
  poll_ms?: number
  timeout_sec?: number
}

export interface StreamCLIExecutionEventsOptions {
  params?: StreamCLIExecutionEventsParams
  signal?: AbortSignal
  onEvent?: (event: CLIExecutionSSEEnvelope) => void
  onMessage?: (event: CLIExecutionEvent) => void
}

export function listCLIExecutions(params?: ListCLIExecutionsParams) {
  return api.get<ListCLIExecutionsResponse>('/cli-executions', { params })
}

export function getCLIExecution(id: string) {
  return api.get<GetCLIExecutionResponse>(`/cli-executions/${encodeURIComponent(id)}`)
}

export function listCLIExecutionEvents(id: string, params?: ListCLIExecutionEventsParams) {
  return api.get<ListCLIExecutionEventsResponse>(`/cli-executions/${encodeURIComponent(id)}/events`, { params })
}

export function listCLIExecutionChildren(id: string, params?: { limit?: number }) {
  return api.get<ListCLIExecutionChildrenResponse>(`/cli-executions/${encodeURIComponent(id)}/children`, { params })
}

export function resumeCLIExecution(id: string, data?: ResumeCLIExecutionRequest) {
  return api.post<ResumeCLIExecutionResponse>(`/cli-executions/${encodeURIComponent(id)}/resume`, data || {})
}

export function buildCLIExecutionEventsStreamURL(id: string, params?: StreamCLIExecutionEventsParams): string {
  const baseURL = typeof api.defaults.baseURL === 'string' ? api.defaults.baseURL : '/api'
  const normalizedBase = baseURL.endsWith('/') ? baseURL.slice(0, -1) : baseURL
  const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
  const url = new URL(`${normalizedBase}/cli-executions/${encodeURIComponent(id)}/stream`, origin)

  const query = new URLSearchParams()
  if (params) {
    if (typeof params.after === 'number' && Number.isFinite(params.after) && params.after >= 0) {
      query.set('after', String(Math.floor(params.after)))
    }
    if (typeof params.limit === 'number' && Number.isFinite(params.limit) && params.limit > 0) {
      query.set('limit', String(Math.floor(params.limit)))
    }
    if (typeof params.poll_ms === 'number' && Number.isFinite(params.poll_ms) && params.poll_ms > 0) {
      query.set('poll_ms', String(Math.floor(params.poll_ms)))
    }
    if (typeof params.timeout_sec === 'number' && Number.isFinite(params.timeout_sec) && params.timeout_sec > 0) {
      query.set('timeout_sec', String(Math.floor(params.timeout_sec)))
    }
  }
  const queryText = query.toString()
  if (queryText) {
    url.search = queryText
  }
  return url.toString()
}

export async function streamCLIExecutionEvents(
  id: string,
  options: StreamCLIExecutionEventsOptions = {}
): Promise<void> {
  const streamURL = buildCLIExecutionEventsStreamURL(id, options.params)
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : ''

  const response = await fetch(streamURL, {
    method: 'GET',
    headers: {
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    signal: options.signal
  })

  if (!response.ok) {
    const body = await response.text().catch(() => '')
    throw new Error(body || `stream request failed with status ${response.status}`)
  }
  if (!response.body) {
    throw new Error('stream response body is empty')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })

    let splitAt = buffer.indexOf('\n\n')
    while (splitAt >= 0) {
      const rawChunk = buffer.slice(0, splitAt)
      buffer = buffer.slice(splitAt + 2)
      splitAt = buffer.indexOf('\n\n')

      const envelope = decodeSSEChunk(rawChunk)
      if (!envelope) {
        continue
      }
      options.onEvent?.(envelope)
      if (envelope.event === 'message') {
        options.onMessage?.(envelope.data as CLIExecutionEvent)
      }
    }
  }
}

function decodeSSEChunk(rawChunk: string): CLIExecutionSSEEnvelope | null {
  const text = rawChunk.replace(/\r/g, '').trim()
  if (!text) {
    return null
  }

  let event: CLIExecutionSSEEventType = 'unknown'
  const dataLines: string[] = []
  for (const line of text.split('\n')) {
    if (!line || line.startsWith(':')) {
      continue
    }
    if (line.startsWith('event:')) {
      event = normalizeSSEEventName(line.slice(6).trim())
      continue
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart())
    }
  }

  if (dataLines.length === 0) {
    return null
  }

  const rawPayload = dataLines.join('\n')
  let data: any = rawPayload
  try {
    data = JSON.parse(rawPayload)
  } catch {
    // keep raw text payload
  }

  return { event, data }
}

function normalizeSSEEventName(raw: string): CLIExecutionSSEEventType {
  switch (raw) {
    case 'ready':
    case 'message':
    case 'done':
    case 'timeout':
    case 'error':
      return raw
    default:
      return 'unknown'
  }
}
