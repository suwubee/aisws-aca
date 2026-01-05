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
        <n-tag v-if="aiStatus" :type="aiStatusType" size="small">
          {{ aiStatus }}
        </n-tag>
      </div>
      <n-button
        size="tiny"
        quaternary
        @click="$emit('open-terminal', task)"
      >
        终端
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Task } from '@/stores/task'

const props = defineProps<{
  task: Task
}>()

const emit = defineEmits<{
  (e: 'edit', task: Task): void
  (e: 'delete', task: Task): void
  (e: 'open-terminal', task: Task): void
}>()

const isDragging = ref(false)

const priorityLabel = computed(() => {
  const labels = ['低', '中', '高', '紧急']
  return labels[props.task.priority] || '低'
})

const priorityType = computed(() => {
  const types = ['default', 'info', 'warning', 'error']
  return types[props.task.priority] as 'default' | 'info' | 'warning' | 'error'
})

const aiStatus = computed(() => {
  // TODO: 从关联的终端获取AI状态
  return null
})

const aiStatusType = computed(() => {
  return 'success'
})

const menuOptions = [
  { label: '编辑', key: 'edit' },
  { label: '删除', key: 'delete' }
]

function handleMenuSelect(key: string) {
  if (key === 'edit') {
    emit('edit', props.task)
  } else if (key === 'delete') {
    emit('delete', props.task)
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
</style>
