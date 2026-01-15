<template>
  <div class="projects-page">
    <div class="page-header">
      <h2>项目管理</h2>
      <p class="page-desc">管理项目与项目集（Portfolio），用于任务/工作流分类与默认执行上下文</p>
    </div>

    <div class="content-area">
      <n-card size="small">
        <ProjectPortfolioManager mode="both" />
      </n-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import ProjectPortfolioManager from '@/components/ProjectPortfolioManager.vue'
</script>

<style scoped>
.projects-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 16px;
}

.page-desc {
  color: #999;
  margin-top: 6px;
  font-size: 13px;
}

.content-area {
  max-width: 1200px;
}

@media (max-width: 768px) {
  .projects-page {
    padding: 12px;
  }
}
</style>

