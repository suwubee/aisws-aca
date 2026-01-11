<template>
  <n-popover
    trigger="click"
    placement="bottom-end"
    :show-arrow="false"
    style="padding: 12px; width: 280px"
  >
    <template #trigger>
      <n-button quaternary size="small" class="context-btn">
        <span class="context-icon">🎯</span>
        <span class="context-label">
          {{ displayLabel }}
        </span>
      </n-button>
    </template>

    <div class="context-panel">
      <n-space vertical :size="10">
        <div class="context-title">
          <n-text strong>当前上下文</n-text>
          <n-button size="tiny" quaternary @click="clearContext" :disabled="!hasContext">
            清除
          </n-button>
        </div>

        <n-select
          v-model:value="groupModel"
          size="small"
          clearable
          filterable
          :loading="projectStore.loadingGroups"
          :options="projectStore.projectGroupOptions"
          placeholder="项目集（可选）"
        />

        <n-select
          v-model:value="projectModel"
          size="small"
          clearable
          filterable
          :loading="projectStore.loadingProjects || projectStore.loadingGroups"
          :options="projectOptions"
          placeholder="项目（可选）"
        />

        <n-text depth="3" style="font-size: 12px">
          用作默认筛选与默认填充（任务/看板/工作流等）。
        </n-text>
      </n-space>
    </div>
  </n-popover>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { NButton, NPopover, NSelect, NSpace, NText } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useGlobalContextStore } from '@/stores/context'
import { useIsMobile } from '@/utils/useIsMobile'

const projectStore = useProjectStore()
const contextStore = useGlobalContextStore()
const { isMobile } = useIsMobile()

const hasContext = computed(() => contextStore.hasContext)

const groupModel = computed<string | null>({
  get: () => contextStore.projectGroupId,
  set: (value) => {
    contextStore.setProjectGroup(value || null)
    if (!contextStore.projectId || !value) return

    const project = projectStore.projects.find(p => p.id === contextStore.projectId)
    const projectGroupId = project?.group_id || null
    if (projectGroupId !== value) {
      contextStore.setProject(null)
    }
  }
})

const projectModel = computed<string | null>({
  get: () => contextStore.projectId,
  set: (value) => {
    const next = value || null
    if (!next) {
      contextStore.setProject(null)
      return
    }

    const project = projectStore.projects.find(p => p.id === next)
    if (project) {
      contextStore.setProject(next, { groupId: project.group_id || null })
    } else {
      contextStore.setProject(next)
    }
  }
})

const projectOptions = computed(() => {
  const groupId = contextStore.projectGroupId
  const base = projectStore.projects
  const filtered = groupId ? base.filter(p => p.group_id === groupId) : base
  return filtered.map(p => {
    const groupName = p.group_id ? projectStore.groupNameMap.get(p.group_id) : null
    return { label: groupName ? `${groupName} / ${p.name}` : p.name, value: p.id }
  })
})

const displayLabel = computed(() => {
  const projectId = contextStore.projectId
  const groupId = contextStore.projectGroupId

  if (projectId) {
    const project = projectStore.projects.find(p => p.id === projectId)
    if (project) {
      const groupName = project.group_id ? projectStore.getProjectGroupName(project.group_id) : null
      const label = groupName ? `${groupName} / ${project.name}` : project.name
      return isMobile.value ? '项目' : label
    }
    return isMobile.value ? '项目' : projectId
  }

  if (groupId) {
    const groupName = projectStore.getProjectGroupName(groupId)
    return isMobile.value ? '项目集' : (groupName || groupId)
  }

  return isMobile.value ? '全部' : '全部项目'
})

function clearContext() {
  contextStore.clear()
}

onMounted(() => {
  projectStore.fetchAll().catch(() => {})
})
</script>

<style scoped>
.context-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: min(320px, 46vw);
}

.context-icon {
  font-size: 16px;
  line-height: 1;
}

.context-label {
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.context-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>

