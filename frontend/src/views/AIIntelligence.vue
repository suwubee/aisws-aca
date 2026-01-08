<template>
  <div class="ai-intelligence">
    <div class="page-header">
      <n-text strong style="font-size: 18px">AI 智能</n-text>
    </div>

    <n-tabs type="line" animated>
      <n-tab-pane name="monitor" tab="代理监控">
        <AgentMonitor />
      </n-tab-pane>
      <n-tab-pane name="stats" tab="性能统计">
        <AgentStats />
      </n-tab-pane>
      <n-tab-pane name="approvals" tab="审批记录">
        <ApprovalCenter />
      </n-tab-pane>
      <n-tab-pane name="logs" tab="思考日志">
        <AIDecisionLog />
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import AgentMonitor from '@/components/AgentMonitor.vue'
import AgentStats from '@/components/AgentStats.vue'
import ApprovalCenter from '@/components/ApprovalCenter.vue'
import AIDecisionLog from '@/components/AIDecisionLog.vue'
</script>

<style scoped>
.ai-intelligence {
  padding: 16px;
  height: calc(100vh - 56px);
  overflow: auto;
}

.page-header {
  margin-bottom: 16px;
}
</style>
