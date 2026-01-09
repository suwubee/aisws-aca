<template>
  <div class="workflow-editor" :class="{ 'workflow-editor--with-config': !!selectedNode }">
    <n-card size="small" class="workflow-editor__sidebar" title="节点">
      <n-space vertical size="small">
        <div
          v-for="item in nodeTypeItems"
          :key="item.type"
          class="node-type"
          :class="`node-type--${item.type}`"
          draggable="true"
          @dragstart="onDragStart($event, item.type)"
        >
          <span class="node-type__dot" />
          <span class="node-type__label">{{ item.label }}</span>
        </div>
      </n-space>

      <n-divider style="margin: 14px 0" />

      <n-space size="small" justify="space-between">
        <n-button size="small" secondary :disabled="loading" @click="loadWorkflow">
          加载
        </n-button>
        <n-button size="small" type="primary" :loading="saving" @click="saveWorkflow">
          保存
        </n-button>
      </n-space>

      <n-text depth="3" style="display: block; margin-top: 10px; font-size: 12px">
        将节点拖到画布上，拖拽连线端点以连接节点
      </n-text>
    </n-card>

    <div
      ref="canvasRef"
      class="workflow-editor__canvas"
      @drop="onDrop"
      @dragover="onDragOver"
    >
      <VueFlow
        v-model:nodes="nodes"
        v-model:edges="edges"
        class="workflow-editor__flow"
        :default-viewport="defaultViewport"
        :min-zoom="0.2"
        :max-zoom="2.5"
        @connect="onConnect"
        @node-click="onNodeClick"
        @pane-click="clearSelection"
      >
        <template #node-server="slotProps">
          <div
            class="workflow-node workflow-node--server"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('server')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '服务器' }}</div>
            <div class="workflow-node__subtitle">server</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('server')" />
          </div>
        </template>

        <template #node-task="slotProps">
          <div
            class="workflow-node workflow-node--task"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('task')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '任务' }}</div>
            <div class="workflow-node__subtitle">task</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('task')" />
          </div>
        </template>

        <template #node-command="slotProps">
          <div
            class="workflow-node workflow-node--command"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('command')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '命令' }}</div>
            <div class="workflow-node__subtitle">command</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('command')" />
          </div>
        </template>

        <template #node-terminal="slotProps">
          <div
            class="workflow-node workflow-node--terminal"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('terminal')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '终端' }}</div>
            <div class="workflow-node__subtitle">terminal</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('terminal')" />
          </div>
        </template>

        <template #node-git="slotProps">
          <div
            class="workflow-node workflow-node--git"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('git')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'Git' }}</div>
            <div class="workflow-node__subtitle">git</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('git')" />
          </div>
        </template>

        <template #node-ops_step="slotProps">
          <div
            class="workflow-node workflow-node--ops-step"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('ops_step')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '步骤' }}</div>
            <div class="workflow-node__subtitle">ops_step</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('ops_step')" />
          </div>
        </template>

        <template #node-ai_agent="slotProps">
          <div
            class="workflow-node workflow-node--ai-agent"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('ai_agent')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'AI代理' }}</div>
            <div class="workflow-node__subtitle">ai_agent</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('ai_agent')" />
          </div>
        </template>

        <template #node-condition="slotProps">
          <div
            class="workflow-node workflow-node--condition"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('condition')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || '条件' }}</div>
            <div class="workflow-node__subtitle">condition</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('condition')" />
          </div>
        </template>
      </VueFlow>

      <div class="workflow-editor__toolbar">
        <n-space size="small">
          <n-button size="small" secondary @click="zoomOut">-</n-button>
          <n-button size="small" secondary @click="zoomIn">+</n-button>
          <n-button size="small" secondary @click="fitToView">适应</n-button>
        </n-space>
      </div>
    </div>

    <n-card v-if="selectedNode" size="small" class="workflow-editor__config" title="配置">
      <n-space vertical size="small">
        <n-space align="center" justify="space-between">
          <n-tag :bordered="false" size="small" :type="tagTypeForNode(selectedNode.type)">
            {{ selectedNode.type }}
          </n-tag>
          <n-space size="small">
            <n-button size="tiny" secondary @click="clearSelection">关闭</n-button>
            <n-button size="tiny" type="error" secondary @click="deleteSelectedNode">删除</n-button>
          </n-space>
        </n-space>

        <n-form label-placement="top" size="small">
          <template v-for="field in fieldsForNode(selectedNode.type, selectedNode.data?.config)" :key="field.key">
            <n-form-item :label="field.label">
              <n-select
                v-if="field.kind === 'select'"
                :value="selectedConfigValue(field.key) || null"
                :options="field.options"
                :loading="field.loading"
                :filterable="field.filterable"
                :clearable="field.clearable"
                :placeholder="field.placeholder"
                @update:value="(value) => updateSelectedConfig(field.key, value)"
              />
              <n-input
                v-else-if="field.kind === 'input' && field.input === 'textarea'"
                type="textarea"
                :autosize="field.autosize || { minRows: 3, maxRows: 8 }"
                :value="selectedConfigValue(field.key)"
                :placeholder="field.placeholder"
                @update:value="(value) => updateSelectedConfig(field.key, value)"
              />
              <n-input
                v-else
                :value="selectedConfigValue(field.key)"
                :placeholder="field.placeholder"
                @update:value="(value) => updateSelectedConfig(field.key, value)"
              />
            </n-form-item>
          </template>
        </n-form>
      </n-space>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Handle, Position, VueFlow, useVueFlow } from '@vue-flow/core'
import type { Connection, Edge, Node, NodeMouseEvent, Viewport } from '@vue-flow/core'
import { useServerStore } from '@/stores/server'
import { getWorkflow, updateWorkflow } from '@/api/workflow'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

type WorkflowNodeType = 'server' | 'task' | 'command' | 'terminal' | 'git' | 'ops_step' | 'ai_agent' | 'condition'

type WorkflowNodeData = {
  label: string
  config: Record<string, string>
}

type WorkflowGraph = {
  nodes: Node<WorkflowNodeData>[]
  edges: Edge[]
  viewport?: Viewport
}

const props = withDefaults(defineProps<{
  workflowId?: string | null
}>(), {
  workflowId: null
})

const emit = defineEmits<{
  (event: 'saved', graph: WorkflowGraph): void
  (event: 'loaded', graph: WorkflowGraph): void
}>()

const message = useMessage()
const serverStore = useServerStore()

const { project, fitView, zoomIn: flowZoomIn, zoomOut: flowZoomOut, setViewport } = useVueFlow()

const canvasRef = ref<HTMLElement | null>(null)

const nodes = ref<Node<WorkflowNodeData>[]>([])
const edges = ref<Edge[]>([])

const selectedNodeId = ref<string | null>(null)

const saving = ref(false)
const loading = ref(false)

const defaultViewport: Viewport = { x: 0, y: 0, zoom: 1 }

const nodeTypeItems: Array<{ type: WorkflowNodeType; label: string; color: string }> = [
  { type: 'server', label: '服务器', color: '#4f8ef7' },
  { type: 'task', label: '任务', color: '#36ad6a' },
  { type: 'command', label: '命令', color: '#5c6bc0' },
  { type: 'terminal', label: '终端', color: '#8c8c8c' },
  { type: 'git', label: 'Git', color: '#f0a020' },
  { type: 'ops_step', label: '步骤', color: '#2dd4bf' },
  { type: 'ai_agent', label: 'AI代理', color: '#b37feb' },
  { type: 'condition', label: '条件', color: '#d03050' }
]

const nodeTypeColorMap = computed(() =>
  Object.fromEntries(nodeTypeItems.map((item) => [item.type, item.color])) as Record<WorkflowNodeType, string>
)

const selectedNode = computed(() => {
  const id = selectedNodeId.value
  if (!id) return null
  return nodes.value.find((node) => node.id === id) || null
})

watch(selectedNode, (node) => {
  if (!selectedNodeId.value) return
  if (!node) selectedNodeId.value = null
})

function storageKey() {
  const id = props.workflowId?.trim()
  return id ? `workflow-graph:${id}` : 'workflow-graph:local'
}

function nodeLabelForType(type: WorkflowNodeType): string {
  const item = nodeTypeItems.find((t) => t.type === type)
  return item?.label || type
}

function defaultConfigForType(type: WorkflowNodeType): Record<string, string> {
  if (type === 'server') return { server_id: '' }
  if (type === 'task') {
    return {
      title: '',
      description: '',
      server_id: '',
      cli_type: 'claude',
      work_dir: '',
      initial_prompt: ''
    }
  }
  if (type === 'command') return { server_id: '', work_dir: '', command: '' }
  if (type === 'terminal') return { server_id: '', work_dir: '', title: '', command: '' }
  if (type === 'git') return { server_id: '', work_dir: '', operation: 'pull', repo_url: '', branch: '', message: '' }
  if (type === 'ops_step') {
    return {
      server_id: '',
      work_dir: '',
      operation: 'none',
      repo_url: '',
      branch: '',
      message: '',
      action: 'command',
      command: '',
      title: '',
      description: '',
      cli_type: 'claude',
      initial_prompt: '',
      auto_create_dir: 'true'
    }
  }
  if (type === 'ai_agent') return { agent_type: 'claude', prompt: '' }
  if (type === 'condition') return { command: '', contains: '', regex: '', description: '' }
  return {}
}

function safeTrim(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function toOneLine(value: string) {
  return value.replace(/\s+/g, ' ').trim()
}

function shorten(value: string, maxLen: number) {
  if (value.length <= maxLen) return value
  return `${value.slice(0, Math.max(0, maxLen - 1))}…`
}

function deriveNodeLabel(type: WorkflowNodeType, config: Record<string, string> | undefined) {
  const cfg = config || {}
  const serverId = safeTrim(cfg.server_id)
  const serverName = serverStore.getServerName(serverId) || (serverId ? serverId : '')

  if (type === 'server') {
    return serverName ? `服务器: ${serverName}` : nodeLabelForType(type)
  }

  if (type === 'task') {
    const title = safeTrim(cfg.title)
    const displayTitle = title ? shorten(title, 24) : '未命名'
    return serverName ? `任务: ${displayTitle} @ ${serverName}` : `任务: ${displayTitle}`
  }

  if (type === 'command') {
    const command = toOneLine(safeTrim(cfg.command))
    const displayCommand = command ? shorten(command, 34) : ''
    const workDir = safeTrim(cfg.work_dir)
    const displayDir = workDir ? shorten(workDir, 18) : ''

    if (serverName && displayCommand) return displayDir ? `命令: ${serverName} · ${displayCommand} · ${displayDir}` : `命令: ${serverName} · ${displayCommand}`
    if (serverName) return displayDir ? `命令: ${serverName} · ${displayDir}` : `命令: ${serverName}`
    if (displayCommand) return displayDir ? `命令: ${displayCommand} · ${displayDir}` : `命令: ${displayCommand}`
    return nodeLabelForType(type)
  }

  if (type === 'terminal') {
    const command = toOneLine(safeTrim(cfg.command))
    const displayCommand = command ? shorten(command, 26) : ''
    const workDir = safeTrim(cfg.work_dir)
    const displayDir = workDir ? shorten(workDir, 18) : ''
    if (serverName && displayCommand) return `终端: ${serverName} · ${displayCommand}`
    if (serverName) return `终端: ${serverName}`
    if (displayCommand) return displayDir ? `终端: ${displayCommand} · ${displayDir}` : `终端: ${displayCommand}`
    return nodeLabelForType(type)
  }

  if (type === 'git') {
    const operation = safeTrim(cfg.operation)
    const repoUrl = safeTrim(cfg.repo_url)
    const branch = safeTrim(cfg.branch)
    const workDir = safeTrim(cfg.work_dir)
    const repoLabel = repoUrl ? shorten(repoUrl.replace(/^https?:\/\//, ''), 26) : ''
    const opLabel = operation || 'git'
    const serverPrefix = serverName ? `${serverName} · ` : ''
    const dirSuffix = workDir ? ` · ${shorten(workDir, 18)}` : ''
    const base = repoLabel ? `Git: ${serverPrefix}${opLabel} ${repoLabel}${dirSuffix}` : `Git: ${serverPrefix}${opLabel}${dirSuffix}`
    return branch ? `${base}#${branch}` : base
  }

  if (type === 'ops_step') {
    const operation = safeTrim(cfg.operation)
    const repoUrl = safeTrim(cfg.repo_url)
    const branch = safeTrim(cfg.branch)
    const message = safeTrim(cfg.message)
    const action = safeTrim(cfg.action) || 'command'
    const command = toOneLine(safeTrim(cfg.command))
    const workDir = safeTrim(cfg.work_dir)

    const parts: string[] = []
    if (serverName) parts.push(serverName)
    if (workDir) parts.push(shorten(workDir, 18))

    if (operation && operation !== 'none') {
      const opPart = repoUrl
        ? `git:${operation} ${shorten(repoUrl.replace(/^https?:\/\//, ''), 22)}${branch ? `#${shorten(branch, 12)}` : ''}`
        : `git:${operation}${branch ? `#${shorten(branch, 12)}` : ''}`
      parts.push(opPart)
      if (operation === 'commit' && message) parts.push(shorten(message, 18))
    }

    if (action === 'task') {
      const title = safeTrim(cfg.title)
      parts.push(title ? `task:${shorten(title, 18)}` : 'task')
    } else if (action === 'terminal') {
      parts.push(command ? `terminal:${shorten(command, 22)}` : 'terminal')
    } else if (action === 'command') {
      parts.push(command ? `cmd:${shorten(command, 22)}` : 'cmd')
    }

    return parts.length > 0 ? `步骤: ${parts.join(' · ')}` : nodeLabelForType(type)
  }

  if (type === 'ai_agent') {
    const agentType = safeTrim(cfg.agent_type)
    return agentType ? `AI代理: ${agentType}` : nodeLabelForType(type)
  }

  if (type === 'condition') {
    const contains = safeTrim(cfg.contains)
    if (contains) return `条件: contains ${shorten(contains, 28)}`
    const regex = safeTrim(cfg.regex)
    if (regex) return `条件: /${shorten(regex, 26)}/`
    const command = toOneLine(safeTrim(cfg.command))
    if (command) return `条件: ${shorten(command, 30)}`
    return nodeLabelForType(type)
  }

  return nodeLabelForType(type)
}

function isWorkflowNodeType(value: unknown): value is WorkflowNodeType {
  return value === 'server' ||
    value === 'task' ||
    value === 'command' ||
    value === 'terminal' ||
    value === 'git' ||
    value === 'ops_step' ||
    value === 'ai_agent' ||
    value === 'condition'
}

function mapToWorkflowNodeType(rawType: unknown): WorkflowNodeType | null {
  if (isWorkflowNodeType(rawType)) return rawType
  if (rawType === 'ai') return 'ai_agent'
  return null
}

function createNodeId() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `node_${Date.now()}_${Math.random().toString(16).slice(2)}`
}

function handleStyle(type: WorkflowNodeType) {
  return { background: nodeTypeColorMap.value[type] }
}

function onDragStart(event: DragEvent, type: WorkflowNodeType) {
  if (!event.dataTransfer) return
  event.dataTransfer.setData('application/vueflow', type)
  event.dataTransfer.effectAllowed = 'move'
}

function onDragOver(event: DragEvent) {
  event.preventDefault()
  if (!event.dataTransfer) return
  event.dataTransfer.dropEffect = 'move'
}

function onDrop(event: DragEvent) {
  event.preventDefault()
  if (!event.dataTransfer || !canvasRef.value) return

  const rawType = event.dataTransfer.getData('application/vueflow')
  const type = mapToWorkflowNodeType(rawType)
  if (!type) return

  const bounds = canvasRef.value.getBoundingClientRect()
  const mousePosition = {
    x: event.clientX - bounds.left,
    y: event.clientY - bounds.top
  }
  const position = typeof project === 'function' ? project(mousePosition) : mousePosition

  const id = createNodeId()
  const config = defaultConfigForType(type)
  nodes.value = [
    ...nodes.value,
    {
      id,
      type,
      position,
      data: {
        label: deriveNodeLabel(type, config),
        config
      }
    }
  ]

  selectedNodeId.value = id
}

function onConnect(connection: Connection) {
  const id = `e_${connection.source}_${connection.target}_${Date.now()}`
  edges.value = [
    ...edges.value,
    {
      id,
      ...connection,
      animated: true,
      style: { stroke: '#7f7f7f' }
    }
  ]
}

function onNodeClick(payload?: NodeMouseEvent | null) {
  const node = payload?.node as Node<WorkflowNodeData> | undefined
  if (!node?.id) return
  selectedNodeId.value = node.id
}

function clearSelection() {
  selectedNodeId.value = null
}

function patchSelectedNodeData(patch: Partial<WorkflowNodeData>) {
  const id = selectedNodeId.value
  if (!id) return

  nodes.value = nodes.value.map((node) => {
    if (node.id !== id) return node
    const nextType = mapToWorkflowNodeType(node.type)
    const config = {
      ...(node.data?.config || {}),
      ...(patch.config || {})
    }
    const nextData: WorkflowNodeData = {
      label: node.data?.label ?? '',
      ...node.data,
      ...patch,
      config
    }
    if (nextType) {
      nextData.label = deriveNodeLabel(nextType, config)
    }
    return { ...node, data: nextData }
  })
}

function selectedConfigValue(key: string) {
  const node = selectedNode.value
  if (!node) return ''
  return node.data?.config?.[key] ?? ''
}

function updateSelectedConfig(key: string, value: string | null) {
  patchSelectedNodeData({ config: { [key]: value ?? '' } })
}

function deleteSelectedNode() {
  const node = selectedNode.value
  if (!node) return

  nodes.value = nodes.value.filter((n) => n.id !== node.id)
  edges.value = edges.value.filter((e) => e.source !== node.id && e.target !== node.id)
  selectedNodeId.value = null
}

type SelectOption = { label: string; value: string }

type NodeField =
  | {
      kind: 'input'
      key: string
      label: string
      input: 'text' | 'textarea'
      placeholder?: string
      autosize?: { minRows: number; maxRows: number }
    }
  | {
      kind: 'select'
      key: string
      label: string
      options: SelectOption[]
      placeholder?: string
      clearable?: boolean
      filterable?: boolean
      loading?: boolean
    }

const cliTypeOptions: SelectOption[] = [
  { label: 'Claude Code', value: 'claude' },
  { label: 'Codex', value: 'codex' },
  { label: 'Gemini CLI', value: 'gemini' }
]

function fieldsForNode(type: string | undefined, config?: Record<string, string>): NodeField[] {
  const normalizedType = mapToWorkflowNodeType(type)
  if (!normalizedType) return []

  if (normalizedType === 'server') {
    return [
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      }
    ]
  }

  if (normalizedType === 'task') {
    return [
      { kind: 'input', key: 'title', label: '标题', input: 'text', placeholder: '任务标题' },
      {
        kind: 'input',
        key: 'description',
        label: '描述',
        input: 'textarea',
        placeholder: '任务描述（可选）',
        autosize: { minRows: 3, maxRows: 6 }
      },
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器（可选）',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      },
      {
        kind: 'select',
        key: 'cli_type',
        label: 'CLI 类型',
        options: cliTypeOptions,
        placeholder: '选择 CLI 工具'
      },
      { kind: 'input', key: 'work_dir', label: '工作目录', input: 'text', placeholder: '/path/to/project' },
      {
        kind: 'input',
        key: 'initial_prompt',
        label: '初始提示',
        input: 'textarea',
        placeholder: '启动后自动输入的提示内容（可选）',
        autosize: { minRows: 3, maxRows: 8 }
      }
    ]
  }

  if (normalizedType === 'command') {
    return [
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器（留空=本机）',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      },
      { kind: 'input', key: 'work_dir', label: '工作目录', input: 'text', placeholder: '/path/to/project（可选）' },
      {
        kind: 'input',
        key: 'command',
        label: '命令',
        input: 'textarea',
        placeholder: '输入要执行的命令（可用于后续条件判断）',
        autosize: { minRows: 3, maxRows: 10 }
      }
    ]
  }

  if (normalizedType === 'terminal') {
    return [
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器（可选）',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      },
      { kind: 'input', key: 'work_dir', label: '工作目录', input: 'text', placeholder: '/path/to/project（可选）' },
      { kind: 'input', key: 'title', label: '会话标题', input: 'text', placeholder: '可选，留空使用节点名称' },
      {
        kind: 'input',
        key: 'command',
        label: '命令',
        input: 'textarea',
        placeholder: '输入要写入终端的命令（偏交互，不建议作为条件判断来源）',
        autosize: { minRows: 3, maxRows: 10 }
      }
    ]
  }

  if (normalizedType === 'git') {
    return [
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器（留空=本机）',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      },
      { kind: 'input', key: 'work_dir', label: '仓库目录', input: 'text', placeholder: '/path/to/repo（pull/push/commit 必填）' },
      {
        kind: 'select',
        key: 'operation',
        label: '操作',
        options: [
          { label: 'clone', value: 'clone' },
          { label: 'pull', value: 'pull' },
          { label: 'push', value: 'push' },
          { label: 'commit', value: 'commit' }
        ],
        placeholder: '选择操作'
      },
      { kind: 'input', key: 'repo_url', label: '仓库地址', input: 'text', placeholder: 'https://...（clone 必填）' },
      { kind: 'input', key: 'branch', label: '分支', input: 'text', placeholder: 'main（可选）' }
    ]
  }

  if (normalizedType === 'ops_step') {
    const action = safeTrim(config?.action).toLowerCase() || 'command'

    const baseFields: NodeField[] = [
      {
        kind: 'select',
        key: 'server_id',
        label: '服务器',
        options: serverStore.serverOptions,
        placeholder: '选择服务器（留空=继承上下文/项目）',
        clearable: true,
        filterable: true,
        loading: serverStore.loading
      },
      { kind: 'input', key: 'work_dir', label: '工作目录', input: 'text', placeholder: '/path/to/project（留空=继承上下文/项目）' },
      {
        kind: 'select',
        key: 'operation',
        label: 'Git 操作',
        options: [
          { label: 'none', value: 'none' },
          { label: 'pull', value: 'pull' },
          { label: 'clone', value: 'clone' },
          { label: 'commit', value: 'commit' },
          { label: 'push', value: 'push' }
        ],
        placeholder: '可选：先执行 git 操作'
      },
      { kind: 'input', key: 'repo_url', label: 'Git 仓库', input: 'text', placeholder: 'https://...（clone 可选，留空尝试继承 Project.git_repo）' },
      { kind: 'input', key: 'branch', label: 'Git 分支', input: 'text', placeholder: 'main（可选，留空尝试继承 Project.git_branch）' },
      { kind: 'input', key: 'message', label: 'Commit 信息', input: 'text', placeholder: 'git commit -m ...（commit 必填）' },
      {
        kind: 'select',
        key: 'action',
        label: '动作',
        options: [
          { label: 'command', value: 'command' },
          { label: 'terminal', value: 'terminal' },
          { label: 'task', value: 'task' },
          { label: 'none', value: 'none' }
        ],
        placeholder: '选择该步骤要执行的动作'
      }
    ]

    if (action === 'task') {
      return [
        ...baseFields,
        { kind: 'input', key: 'title', label: '任务标题', input: 'text', placeholder: '可选，留空使用节点名称' },
        {
          kind: 'select',
          key: 'cli_type',
          label: 'CLI 类型',
          options: cliTypeOptions,
          placeholder: '选择 CLI 工具'
        },
        { kind: 'input', key: 'initial_prompt', label: '初始提示', input: 'textarea', placeholder: '可选：启动后自动输入的提示内容' },
        {
          kind: 'select',
          key: 'auto_create_dir',
          label: '自动创建目录',
          options: [
            { label: 'true', value: 'true' },
            { label: 'false', value: 'false' }
          ],
          placeholder: '默认 true'
        }
      ]
    }

    if (action === 'terminal') {
      return [
        ...baseFields,
        { kind: 'input', key: 'title', label: '会话标题', input: 'text', placeholder: '可选，留空使用节点名称' },
        { kind: 'input', key: 'command', label: '命令', input: 'textarea', placeholder: '写入终端的命令（偏交互）', autosize: { minRows: 3, maxRows: 10 } }
      ]
    }

    if (action === 'command') {
      return [
        ...baseFields,
        { kind: 'input', key: 'command', label: '命令', input: 'textarea', placeholder: '执行命令（可用于后续条件判断）', autosize: { minRows: 3, maxRows: 10 } }
      ]
    }

    return baseFields
  }

  if (normalizedType === 'ai_agent') {
    return [
      {
        kind: 'select',
        key: 'agent_type',
        label: 'Agent 类型',
        options: cliTypeOptions,
        placeholder: '选择 Agent'
      },
      {
        kind: 'input',
        key: 'prompt',
        label: '提示词',
        input: 'textarea',
        placeholder: '输入要发送给 Agent 的提示',
        autosize: { minRows: 4, maxRows: 10 }
      }
    ]
  }

  if (normalizedType === 'condition') {
    return [
      {
        kind: 'input',
        key: 'command',
        label: '检测命令',
        input: 'textarea',
        placeholder: '退出码=0 视为 true；可配合 contains/regex 对输出做判断',
        autosize: { minRows: 3, maxRows: 10 }
      },
      { kind: 'input', key: 'contains', label: '输出包含', input: 'text', placeholder: '可选：stdout/stderr 包含该文本则 true' },
      { kind: 'input', key: 'regex', label: '输出正则', input: 'text', placeholder: '可选：正则匹配输出则 true' },
      {
        kind: 'input',
        key: 'description',
        label: '描述',
        input: 'textarea',
        placeholder: '条件说明（可选）',
        autosize: { minRows: 3, maxRows: 6 }
      }
    ]
  }

  return []
}

function tagTypeForNode(type: string | undefined) {
  if (type === 'task') return 'success'
  if (type === 'server') return 'info'
  if (type === 'command') return 'info'
  if (type === 'terminal') return 'default'
  if (type === 'git') return 'warning'
  if (type === 'ops_step') return 'info'
  if (type === 'ai_agent') return 'warning'
  if (type === 'condition') return 'error'
  return 'default'
}

function normalizeNode(raw: any): Node<WorkflowNodeData> | null {
  if (!raw || typeof raw !== 'object') return null
  const id = typeof raw.id === 'string' ? raw.id : String(raw.id || '')
  if (!id) return null

  const type = mapToWorkflowNodeType(raw.type)
  if (!type) {
    const label = typeof raw.data?.label === 'string' ? raw.data.label : String(raw.type || '')
    return {
      ...raw,
      id,
      data: {
        label,
        config: {}
      }
    } as Node<WorkflowNodeData>
  }

  const baseConfig = defaultConfigForType(type)
  const rawConfig = raw.data?.config && typeof raw.data.config === 'object' ? raw.data.config : {}
  const config: Record<string, string> = {
    ...baseConfig,
    ...Object.fromEntries(Object.entries(rawConfig).map(([key, value]) => [key, typeof value === 'string' ? value : String(value ?? '')]))
  }

  if (raw.type === 'ai') {
    if (!safeTrim(config.agent_type) && safeTrim((rawConfig as any).model)) {
      config.agent_type = safeTrim((rawConfig as any).model)
    }
  }

  if (raw.type === 'task') {
    if (!safeTrim(config.title) && safeTrim((rawConfig as any).task_id)) {
      config.title = safeTrim((rawConfig as any).task_id)
    }
  }

  return {
    ...raw,
    id,
    type,
    data: {
      label: deriveNodeLabel(type, config),
      config
    }
  } as Node<WorkflowNodeData>
}

function refreshAllNodeLabels() {
  nodes.value = nodes.value.map((node) => {
    const type = mapToWorkflowNodeType(node.type)
    if (!type) return node
    const config = {
      ...defaultConfigForType(type),
      ...(node.data?.config || {})
    }
    return {
      ...node,
      type,
      data: {
        label: deriveNodeLabel(type, config),
        config
      }
    }
  })
}

function getGraphSnapshot(): WorkflowGraph {
  const base: WorkflowGraph = { nodes: nodes.value, edges: edges.value }
  return base
}

function safeParseJSONArray(raw: unknown): unknown[] {
  if (Array.isArray(raw)) return raw
  if (typeof raw !== 'string') return []
  const trimmed = raw.trim()
  if (!trimmed) return []
  try {
    const parsed = JSON.parse(trimmed)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

async function saveWorkflow() {
  if (saving.value) return
  saving.value = true
  try {
    const snapshot = getGraphSnapshot()
    localStorage.setItem(storageKey(), JSON.stringify(snapshot))

    const id = props.workflowId?.trim()
    if (id) {
      await updateWorkflow(id, {
        nodes: JSON.stringify(nodes.value),
        edges: JSON.stringify(edges.value)
      })
      message.success('工作流已保存到服务器')
    } else {
      message.success('工作流已保存到本地')
    }
    emit('saved', snapshot)
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function loadWorkflow(options: { silent?: boolean } = {}) {
  if (loading.value) return
  loading.value = true
  try {
    const id = props.workflowId?.trim()

    let parsed: WorkflowGraph | null = null
    if (id) {
      try {
        const { data } = await getWorkflow(id)
        const item = data?.item
        const serverNodes = safeParseJSONArray(item?.nodes)
        const serverEdges = safeParseJSONArray(item?.edges)
        parsed = {
          nodes: serverNodes as any[],
          edges: serverEdges as any[]
        }

        const isEmptyServerGraph = serverNodes.length === 0 && serverEdges.length === 0
        if (isEmptyServerGraph) {
          const cached = localStorage.getItem(storageKey())
          if (cached) {
            parsed = JSON.parse(cached) as WorkflowGraph
          }
        }
      } catch (e: any) {
        const cached = localStorage.getItem(storageKey())
        if (cached) {
          parsed = JSON.parse(cached) as WorkflowGraph
        } else {
          throw e
        }
      }
    } else {
      const raw = localStorage.getItem(storageKey())
      if (!raw) {
        if (!options.silent) message.info('暂无已保存的工作流')
        return
      }
      parsed = JSON.parse(raw) as WorkflowGraph
    }

    if (!parsed) {
      if (!options.silent) message.info('暂无已保存的工作流')
      return
    }

    nodes.value = Array.isArray(parsed.nodes)
      ? (parsed.nodes.map(normalizeNode).filter(Boolean) as Node<WorkflowNodeData>[])
      : []
    edges.value = Array.isArray(parsed.edges) ? parsed.edges : []
    selectedNodeId.value = null
    refreshAllNodeLabels()

    await nextTick()
    if (parsed.viewport && typeof setViewport === 'function') {
      setViewport(parsed.viewport)
    } else if (typeof fitView === 'function') {
      fitView()
    }
    emit('loaded', parsed)
    if (!options.silent) message.success('工作流已加载')
  } catch (e: any) {
    if (!options.silent) message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function zoomIn() {
  if (typeof flowZoomIn === 'function') flowZoomIn()
}

function zoomOut() {
  if (typeof flowZoomOut === 'function') flowZoomOut()
}

function fitToView() {
  if (typeof fitView === 'function') fitView()
}

watch(() => props.workflowId, () => {
  nodes.value = []
  edges.value = []
  selectedNodeId.value = null
  loadWorkflow({ silent: true })
}, { immediate: true })

watch(() => serverStore.servers, () => {
  refreshAllNodeLabels()
})

onMounted(() => {
  serverStore.fetchServers().catch(() => {
    message.warning('加载服务器列表失败')
  })
})
</script>

<style scoped>
.workflow-editor {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 12px;
  height: 100%;
  min-height: 560px;
}

.workflow-editor--with-config {
  grid-template-columns: 240px minmax(0, 1fr) 300px;
}

.workflow-editor__sidebar {
  height: 100%;
  overflow: auto;
}

.node-type {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  cursor: grab;
  user-select: none;
}

.node-type:active {
  cursor: grabbing;
}

.node-type__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: currentColor;
}

.node-type--server {
  color: #4f8ef7;
}

.node-type--task {
  color: #36ad6a;
}

.node-type--command {
  color: #5c6bc0;
}

.node-type--terminal {
  color: #8c8c8c;
}

.node-type--git {
  color: #f0a020;
}

.node-type--ops_step {
  color: #2dd4bf;
}

.node-type--ai_agent {
  color: #b37feb;
}

.node-type--condition {
  color: #d03050;
}

.node-type__label {
  font-size: 13px;
  font-weight: 600;
  color: #e5e5e5;
}

.workflow-editor__canvas {
  position: relative;
  height: 100%;
  min-height: 560px;
  border-radius: 12px;
  overflow: hidden;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.03), rgba(255, 255, 255, 0.02));
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.workflow-editor__flow {
  height: 100%;
}

.workflow-editor__toolbar {
  position: absolute;
  left: 12px;
  top: 12px;
  z-index: 10;
}

.workflow-editor__config {
  height: 100%;
  overflow: auto;
}

.workflow-node {
  position: relative;
  min-width: 140px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(22, 22, 22, 0.72);
  color: #e6e6e6;
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.25);
}

.workflow-node--selected {
  box-shadow: 0 0 0 2px rgba(120, 120, 120, 0.4), 0 10px 24px rgba(0, 0, 0, 0.25);
}

.workflow-node__title {
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.workflow-node__subtitle {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
}

.workflow-node--server {
  border-color: rgba(79, 142, 247, 0.7);
}

.workflow-node--task {
  border-color: rgba(54, 173, 106, 0.7);
}

.workflow-node--command {
  border-color: rgba(92, 107, 192, 0.75);
}

.workflow-node--terminal {
  border-color: rgba(140, 140, 140, 0.7);
}

.workflow-node--git {
  border-color: rgba(240, 160, 32, 0.75);
}

.workflow-node--ops-step {
  border-color: rgba(45, 212, 191, 0.75);
}

.workflow-node--ai-agent {
  border-color: rgba(179, 127, 235, 0.7);
}

.workflow-node--condition {
  border-color: rgba(208, 48, 80, 0.75);
}

.workflow-editor :deep(.vue-flow__handle) {
  width: 10px;
  height: 10px;
  border: 2px solid rgba(0, 0, 0, 0.35);
}

.workflow-editor :deep(.vue-flow__edge-path) {
  stroke-width: 2.5;
}
</style>
