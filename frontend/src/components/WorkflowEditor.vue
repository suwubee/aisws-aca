<template>
  <div class="workflow-editor">
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
        <template #node-command="slotProps">
          <div
            class="workflow-node workflow-node--command"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('command')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'Command' }}</div>
            <div class="workflow-node__subtitle">command</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('command')" />
          </div>
        </template>

        <template #node-task="slotProps">
          <div
            class="workflow-node workflow-node--task"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('task')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'Task' }}</div>
            <div class="workflow-node__subtitle">task</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('task')" />
          </div>
        </template>

        <template #node-ai="slotProps">
          <div
            class="workflow-node workflow-node--ai"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('ai')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'AI' }}</div>
            <div class="workflow-node__subtitle">ai</div>
            <Handle type="source" :position="Position.Right" :style="handleStyle('ai')" />
          </div>
        </template>

        <template #node-condition="slotProps">
          <div
            class="workflow-node workflow-node--condition"
            :class="{ 'workflow-node--selected': slotProps.selected }"
          >
            <Handle type="target" :position="Position.Left" :style="handleStyle('condition')" />
            <div class="workflow-node__title">{{ slotProps.data?.label || 'Condition' }}</div>
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

    <n-card size="small" class="workflow-editor__config" title="配置">
      <n-alert v-if="!selectedNode" type="info" :bordered="false">
        点击节点以编辑配置
      </n-alert>

      <template v-else>
        <n-space vertical size="small">
          <n-space align="center" justify="space-between">
            <n-tag :bordered="false" size="small" :type="tagTypeForNode(selectedNode.type)">
              {{ selectedNode.type }}
            </n-tag>
            <n-button size="tiny" type="error" secondary @click="deleteSelectedNode">
              删除
            </n-button>
          </n-space>

          <n-form label-placement="top" size="small">
            <n-form-item label="名称">
              <n-input
                :value="selectedNode.data?.label || ''"
                placeholder="节点名称"
                @update:value="updateSelectedLabel"
              />
            </n-form-item>

            <template v-for="field in fieldsForNode(selectedNode.type)" :key="field.key">
              <n-form-item :label="field.label">
                <n-input
                  v-if="field.input === 'textarea'"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 8 }"
                  :value="selectedConfigValue(field.key)"
                  @update:value="(value) => updateSelectedConfig(field.key, value)"
                />
                <n-input
                  v-else
                  :value="selectedConfigValue(field.key)"
                  @update:value="(value) => updateSelectedConfig(field.key, value)"
                />
              </n-form-item>
            </template>
          </n-form>
        </n-space>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Handle, Position, VueFlow, useVueFlow } from '@vue-flow/core'
import type { Connection, Edge, Node, Viewport } from '@vue-flow/core'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

type WorkflowNodeType = 'command' | 'task' | 'ai' | 'condition'

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

const { project, fitView, zoomIn: flowZoomIn, zoomOut: flowZoomOut, setViewport } = useVueFlow()

const canvasRef = ref<HTMLElement | null>(null)

const nodes = ref<Node<WorkflowNodeData>[]>([])
const edges = ref<Edge[]>([])

const selectedNodeId = ref<string | null>(null)

const saving = ref(false)
const loading = ref(false)

const defaultViewport: Viewport = { x: 0, y: 0, zoom: 1 }

const nodeTypeItems: Array<{ type: WorkflowNodeType; label: string; color: string }> = [
  { type: 'command', label: 'Command', color: '#4f8ef7' },
  { type: 'task', label: 'Task', color: '#36ad6a' },
  { type: 'ai', label: 'AI', color: '#b37feb' },
  { type: 'condition', label: 'Condition', color: '#f0a020' }
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
  if (type === 'command') return { command: '' }
  if (type === 'task') return { task_id: '' }
  if (type === 'ai') return { prompt: '', model: '' }
  if (type === 'condition') return { expression: '' }
  return {}
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

  const type = event.dataTransfer.getData('application/vueflow') as WorkflowNodeType
  if (!type) return

  const bounds = canvasRef.value.getBoundingClientRect()
  const mousePosition = {
    x: event.clientX - bounds.left,
    y: event.clientY - bounds.top
  }
  const position = typeof project === 'function' ? project(mousePosition) : mousePosition

  const id = createNodeId()
  nodes.value = [
    ...nodes.value,
    {
      id,
      type,
      position,
      data: {
        label: nodeLabelForType(type),
        config: defaultConfigForType(type)
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

function onNodeClick(_: MouseEvent, node: Node<WorkflowNodeData>) {
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
    const nextData: WorkflowNodeData = {
      label: node.data?.label ?? '',
      ...node.data,
      ...patch,
      config: {
        ...(node.data?.config || {}),
        ...(patch.config || {})
      }
    }
    return { ...node, data: nextData }
  })
}

function updateSelectedLabel(value: string) {
  patchSelectedNodeData({ label: value })
}

function selectedConfigValue(key: string) {
  const node = selectedNode.value
  if (!node) return ''
  return node.data?.config?.[key] ?? ''
}

function updateSelectedConfig(key: string, value: string) {
  patchSelectedNodeData({ config: { [key]: value } })
}

function deleteSelectedNode() {
  const node = selectedNode.value
  if (!node) return

  nodes.value = nodes.value.filter((n) => n.id !== node.id)
  edges.value = edges.value.filter((e) => e.source !== node.id && e.target !== node.id)
  selectedNodeId.value = null
}

type NodeField = { key: string; label: string; input: 'text' | 'textarea' }

const nodeFields: Record<WorkflowNodeType, NodeField[]> = {
  command: [{ key: 'command', label: '命令', input: 'textarea' }],
  task: [{ key: 'task_id', label: '任务ID', input: 'text' }],
  ai: [
    { key: 'model', label: '模型', input: 'text' },
    { key: 'prompt', label: '提示词', input: 'textarea' }
  ],
  condition: [{ key: 'expression', label: '条件表达式', input: 'text' }]
}

function fieldsForNode(type: WorkflowNodeType | undefined) {
  if (!type) return []
  return nodeFields[type] || []
}

function tagTypeForNode(type: string | undefined) {
  if (type === 'task') return 'success'
  if (type === 'command') return 'info'
  if (type === 'ai') return 'warning'
  if (type === 'condition') return 'error'
  return 'default'
}

function getGraphSnapshot(): WorkflowGraph {
  const base: WorkflowGraph = { nodes: nodes.value, edges: edges.value }
  return base
}

async function saveWorkflow() {
  if (saving.value) return
  saving.value = true
  try {
    const snapshot = getGraphSnapshot()
    localStorage.setItem(storageKey(), JSON.stringify(snapshot))
    emit('saved', snapshot)
    message.success('工作流已保存')
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
    const raw = localStorage.getItem(storageKey())
    if (!raw) {
      if (!options.silent) message.info('暂无已保存的工作流')
      return
    }
    const parsed = JSON.parse(raw) as WorkflowGraph
    nodes.value = Array.isArray(parsed.nodes) ? parsed.nodes : []
    edges.value = Array.isArray(parsed.edges) ? parsed.edges : []
    selectedNodeId.value = null

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
</script>

<style scoped>
.workflow-editor {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr) 300px;
  gap: 12px;
  height: 100%;
  min-height: 560px;
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

.node-type--command {
  color: #4f8ef7;
}

.node-type--task {
  color: #36ad6a;
}

.node-type--ai {
  color: #b37feb;
}

.node-type--condition {
  color: #f0a020;
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

.workflow-node--command {
  border-color: rgba(79, 142, 247, 0.7);
}

.workflow-node--task {
  border-color: rgba(54, 173, 106, 0.7);
}

.workflow-node--ai {
  border-color: rgba(179, 127, 235, 0.7);
}

.workflow-node--condition {
  border-color: rgba(240, 160, 32, 0.75);
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
