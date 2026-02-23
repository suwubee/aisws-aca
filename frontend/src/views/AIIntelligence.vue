<template>
  <div class="ai-intelligence">
    <div class="page-header">
      <n-text strong style="font-size: 18px">AI 智能</n-text>
    </div>

    <n-tabs v-model:value="activeTab" type="line" animated>
      <n-tab-pane name="history" tab="执行历史">
        <WorkflowHistoryList session-jump-target="workflow" />
      </n-tab-pane>
      <n-tab-pane name="monitor" tab="代理监控">
        <AgentMonitor />
      </n-tab-pane>
      <n-tab-pane name="stats" tab="性能统计">
        <AgentStats />
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AgentMonitor from '@/components/AgentMonitor.vue'
import AgentStats from '@/components/AgentStats.vue'
import WorkflowHistoryList from '@/components/WorkflowHistoryList.vue'

const route = useRoute()
const router = useRouter()

function normalizeTab(raw: unknown): 'history' | 'monitor' | 'stats' {
  const tab = String(raw || '').trim().toLowerCase()
  if (tab === 'monitor' || tab === 'stats' || tab === 'history') return tab
  return 'history'
}

const activeTab = ref<'history' | 'monitor' | 'stats'>(normalizeTab(route.query.tab))

watch(
  () => route.query.tab,
  (tab) => {
    const next = normalizeTab(tab)
    if (next !== activeTab.value) {
      activeTab.value = next
    }
  }
)

watch(activeTab, (tab) => {
  const next = normalizeTab(tab)
  if (normalizeTab(route.query.tab) === next) return
  void router.replace({
    query: {
      ...route.query,
      tab: next
    }
  })
})
</script>

<style scoped>
.ai-intelligence {
  padding: 16px;
  height: calc(100vh - var(--app-header-height) - var(--app-bottom-nav-height));
  overflow: auto;
}

.page-header {
  margin-bottom: 16px;
}
</style>
