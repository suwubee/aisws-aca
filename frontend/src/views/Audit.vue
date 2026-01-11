<template>
  <div class="audit-page">
    <div class="page-header">
      <h2>审计</h2>
      <p class="page-desc">集中查看日志、审批、登录等审计信息</p>
    </div>

    <div class="content-area">
      <n-tabs v-model:value="activeTab" :type="isMobile ? 'segment' : 'line'" animated size="small">
        <n-tab-pane name="terminal-logs" tab="终端日志">
          <LogManagement embedded />
        </n-tab-pane>
        <n-tab-pane name="approvals" tab="审批记录">
          <ApprovalRecords />
        </n-tab-pane>
        <n-tab-pane name="ai-decisions" tab="AI 决策">
          <AIDecisionLog />
        </n-tab-pane>
        <n-tab-pane name="login" tab="登录记录">
          <LoginRecords />
        </n-tab-pane>
      </n-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NTabPane, NTabs } from 'naive-ui'
import { useIsMobile } from '@/utils/useIsMobile'
import LogManagement from '@/views/LogManagement.vue'
import ApprovalRecords from '@/components/ApprovalRecords.vue'
import AIDecisionLog from '@/components/AIDecisionLog.vue'
import LoginRecords from '@/components/LoginRecords.vue'

const activeTab = ref<'terminal-logs' | 'approvals' | 'ai-decisions' | 'login'>('terminal-logs')
const { isMobile } = useIsMobile()
</script>

<style scoped>
.audit-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  padding: 20px 24px;
  border-bottom: 1px solid #333;
  background: #252525;
}

.page-header h2 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
  color: #e0e0e0;
}

.page-desc {
  margin: 0;
  font-size: 13px;
  color: #888;
}

.content-area {
  flex: 1;
  padding: 16px;
  overflow: hidden;
}

@media (max-width: 768px) {
  .page-header {
    padding: 14px 14px;
  }

  .content-area {
    padding: 12px;
  }
}
</style>

