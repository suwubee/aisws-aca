<template>
  <div class="schedules-page">
    <div class="page-header">
      <h2>计划任务</h2>
      <p class="page-desc">定时运行任务或 AI 工作流（cron / 单次）</p>
    </div>

    <div class="content-area">
      <ScheduledJobs />
    </div>
  </div>
</template>

<script setup lang="ts">
import ScheduledJobs from '@/components/ScheduledJobs.vue'
</script>

<style scoped>
.schedules-page {
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

