<template>
  <div
    class="task-card"
    :class="{ dragging: isDragging }"
    @dragstart="isDragging = true"
    @dragend="isDragging = false"
  >
    <div class="task-header">
      <n-text strong>{{ task.title }}</n-text>
      <n-dropdown :options="menuOptions" @select="handleMenuSelect">
        <n-button quaternary size="tiny">
          <template #icon>
            <span>⋮</span>
          </template>
        </n-button>
      </n-dropdown>
    </div>

    <div v-if="task.description" class="task-description">
      <n-ellipsis :line-clamp="2">
        {{ task.description }}
      </n-ellipsis>
    </div>

    <div class="task-footer">
      <div class="task-meta">
        <n-tag
          v-if="task.priority > 0"
          :type="priorityType"
          size="small"
        >
          {{ priorityLabel }}
        </n-tag>
        <n-tag v-if="serverLabel" size="small">
          {{ serverLabel }}
        </n-tag>
        <n-tag v-if="task.cli_type" size="small" type="info">
          {{ task.cli_type }}
        </n-tag>
        <n-tag v-if="aiStatus" :type="aiStatusType" size="small">
          {{ aiStatus }}
        </n-tag>
      </div>
      <n-popover
        v-if="terminalCount > 0"
        trigger="hover"
        placement="bottom-end"
        :show-arrow="false"
        style="max-width: 280px"
      >
        <template #trigger>
          <n-button size="tiny" quaternary @click="$emit('open-terminal', task)">
            终端
            <n-badge :value="terminalCount" :max="99" class="terminal-badge" />
          </n-button>
        </template>
        <div class="terminal-popover">
          <div class="terminal-popover-title">关联终端</div>
          <div
            v-for="t in relatedTerminals"
            :key="t.id"
            class="terminal-popover-item"
            @click="activateTerminal(t.id)"
          >
            <span class="terminal-name">{{ t.title || 'Terminal' }}</span>
            <span class="terminal-state">{{ t.status }}</span>
          </div>
          <n-divider style="margin: 8px 0" />
          <div class="terminal-popover-item terminal-popover-new" @click="$emit('open-terminal', task)">
            + 新建终端
          </div>
        </div>
      </n-popover>
      <n-button v-else size="tiny" quaternary @click="$emit('open-terminal', task)">
        终端
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Task } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useServerStore } from '@/stores/server'

const props = defineProps<{
  task: Task
}>()

const emit = defineEmits<{
  (e: 'edit', task: Task): void
  (e: 'delete', task: Task): void
  (e: 'open-terminal', task: Task): void
  (e: 'start', task: Task): void
  (e: 'detail', task: Task): void
}>()

const isDragging = ref(false)
const terminalStore = useTerminalStore()
const serverStore = useServerStore()

const relatedTerminals = computed(() =>
  terminalStore.terminals.filter(t => t.task_id === props.task.id)
)

const terminalCount = computed(() => relatedTerminals.value.length)

const priorityLabel = computed(() => {
  const labels = ['低', '中', '高', '紧急']
  return labels[props.task.priority] || '低'
})

const priorityType = computed(() => {
  const types = ['default', 'info', 'warning', 'error']
  return types[props.task.priority] as 'default' | 'info' | 'warning' | 'error'
})

const aiStatus = computed(() => {
  // 从关联的终端获取AI状态
  const terminal = relatedTerminals.value.find(t => t.metadata?.ai_assistant?.detected)
  if (terminal?.metadata?.ai_assistant) {
    return terminal.metadata.ai_assistant.state
  }
  return null
})

const aiStatusType = computed(() => {
  const status = aiStatus.value
  if (status === 'idle') return 'default'
  if (status === 'thinking') return 'warning'
  if (status === 'executing') return 'info'
  if (status === 'waiting_approval') return 'error'
  return 'success'
})

const serverLabel = computed(() => {
  if (props.task.server?.name) return props.task.server.name
  if (!props.task.server_id) return null
  return serverStore.getServerName(props.task.server_id) || props.task.server_id
})

function activateTerminal(id: string) {
  terminalStore.setActiveTerminal(id)
}

const menuOptions = computed(() => {
  const options = [
    { label: '编辑', key: 'edit' },
    { label: '查看详情', key: 'detail' }
  ]
  // 如果有自动化配置，添加启动选项
  if (props.task.work_dir || props.task.cli_type) {
    options.unshift({ label: '▶ 启动任务', key: 'start' })
  }
  options.push({ label: '删除', key: 'delete' })
  return options
})

function handleMenuSelect(key: string) {
  if (key === 'edit') {
    emit('edit', props.task)
  } else if (key === 'delete') {
    emit('delete', props.task)
  } else if (key === 'start') {
    emit('start', props.task)
  } else if (key === 'detail') {
    emit('detail', props.task)
  }
}
</script>

<style scoped>
.task-card {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  cursor: grab;
  transition: all 0.2s;
}

.task-card:hover {
  border-color: var(--primary-color);
  transform: translateY(-2px);
}

.task-card.dragging {
  opacity: 0.5;
  cursor: grabbing;
}

.task-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 8px;
}

.task-description {
  color: #888;
  font-size: 13px;
  margin-bottom: 8px;
}

.task-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.task-meta {
  display: flex;
  gap: 4px;
}

.terminal-badge {
  margin-left: 6px;
}

.terminal-popover-title {
  font-size: 12px;
  color: #aaa;
  margin-bottom: 6px;
}

.terminal-popover-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  user-select: none;
}

.terminal-popover-item:hover {
  background: rgba(255, 255, 255, 0.06);
}

.terminal-popover-new {
  color: var(--primary-color);
  justify-content: center;
}

.terminal-name {
  max-width: 190px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-state {
  font-size: 12px;
  color: #888;
}
</style>
