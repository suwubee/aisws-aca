<template>
  <n-card class="ai-decision-display" size="small" :bordered="false">
    <template #header>
      <div class="header">
        <n-text strong>AI 决策结果</n-text>
        <div class="header-tags">
          <n-tag size="small" :bordered="false" :type="actionTagType">
            {{ actionText }}
          </n-tag>
          <n-tag v-if="decision.ai_decision" size="small" :bordered="false" type="info">
            AI
          </n-tag>
        </div>
      </div>
    </template>

    <div class="content">
      <div class="row">
        <n-text depth="3">置信度</n-text>
        <div class="confidence">
          <n-progress
            type="line"
            :percentage="confidencePercent"
            :show-indicator="false"
            :status="confidenceStatus"
          />
          <n-text depth="2">{{ confidencePercent }}%</n-text>
        </div>
      </div>

      <div class="row column">
        <n-text depth="3">匹配规则</n-text>
        <n-text class="value" depth="2">
          {{ ruleMatchedText }}
        </n-text>
      </div>

      <div class="row column">
        <n-text depth="3">理由</n-text>
        <n-text class="value reasoning" depth="2">
          {{ reasoningText }}
        </n-text>
      </div>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'

type DecisionAction = 'approve' | 'reject' | 'wait' | 'input' | (string & {})

interface AIDecision {
  action: DecisionAction
  confidence: number
  reasoning: string
  rule_matched: string
  ai_decision: boolean
}

type TagType = 'default' | 'info' | 'success' | 'warning' | 'error'
type ProgressStatus = 'default' | 'success' | 'error' | 'warning'

const props = defineProps<{
  decision: AIDecision
}>()

const normalizedAction = computed(() => (props.decision.action || '').trim().toLowerCase())

const actionTagType = computed<TagType>(() => {
  if (
    normalizedAction.value === 'approve' ||
    normalizedAction.value === 'y' ||
    normalizedAction.value === 'yes' ||
    normalizedAction.value === '通过'
  ) return 'success'
  if (
    normalizedAction.value === 'reject' ||
    normalizedAction.value === 'n' ||
    normalizedAction.value === 'no' ||
    normalizedAction.value === '拒绝'
  ) return 'error'
  if (normalizedAction.value === 'wait' || normalizedAction.value === '等待') return 'warning'
  if (normalizedAction.value === 'input' || normalizedAction.value === '需要输入') return 'info'
  return 'default'
})

const actionText = computed(() => {
  if (
    normalizedAction.value === 'approve' ||
    normalizedAction.value === 'y' ||
    normalizedAction.value === 'yes' ||
    normalizedAction.value === '通过'
  ) return '通过 (approve)'
  if (
    normalizedAction.value === 'reject' ||
    normalizedAction.value === 'n' ||
    normalizedAction.value === 'no' ||
    normalizedAction.value === '拒绝'
  ) return '拒绝 (reject)'
  if (normalizedAction.value === 'wait' || normalizedAction.value === '等待') return '等待 (wait)'
  if (normalizedAction.value === 'input' || normalizedAction.value === '需要输入') return '需要输入 (input)'
  return props.decision.action || '—'
})

const normalizedConfidence = computed(() => {
  const value = props.decision.confidence
  if (!Number.isFinite(value)) return null

  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return null

  const normalized = numeric > 1 && numeric <= 100 ? numeric / 100 : numeric
  return Math.max(0, Math.min(1, normalized))
})

const confidencePercent = computed(() => {
  const value = normalizedConfidence.value
  if (value == null) return 0
  return Math.round(value * 100)
})

const confidenceStatus = computed<ProgressStatus>(() => {
  const value = normalizedConfidence.value
  if (value == null) return 'default'

  const percent = confidencePercent.value
  if (percent >= 80) return 'success'
  if (percent >= 50) return 'warning'
  return 'error'
})

const reasoningText = computed(() => {
  const text = (props.decision.reasoning || '').trim()
  return text || '—'
})

const ruleMatchedText = computed(() => {
  const text = (props.decision.rule_matched || '').trim()
  return text || '—'
})
</script>

<style scoped>
.ai-decision-display {
  background: #1e1e1e;
  border: 1px solid #333;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.header-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.row.column {
  align-items: flex-start;
  flex-direction: column;
}

.confidence {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 220px;
}

.confidence :deep(.n-progress) {
  flex: 1;
  min-width: 140px;
}

.value {
  width: 100%;
  word-break: break-word;
  white-space: pre-wrap;
}

.reasoning {
  line-height: 1.5;
}
</style>
