<template>
  <n-modal
    :show="show"
    preset="card"
    :title="'终端规则配置 - ' + (terminalTitle || terminalId)"
    style="width: 650px"
    :bordered="false"
    @close="emit('close')"
  >
    <n-spin :show="loading">
      <n-form label-placement="left" label-width="120">
        <n-form-item label="规则模式">
          <n-radio-group v-model:value="ruleMode">
            <n-space vertical>
              <n-radio value="none">
                <n-space align="center">
                  <span>不使用规则</span>
                  <n-text depth="3">默认，不自动处理任何审批</n-text>
                </n-space>
              </n-radio>
              <n-radio value="system">
                <n-space align="center">
                  <span>使用系统规则</span>
                  <n-text depth="3">继承系统设置中的审批规则</n-text>
                </n-space>
              </n-radio>
              <n-radio value="task" :disabled="!hasTask">
                <n-space align="center">
                  <span>使用任务规则</span>
                  <n-text depth="3">{{ hasTask ? '继承关联任务的审批规则' : '终端未关联任务' }}</n-text>
                </n-space>
              </n-radio>
              <n-radio value="custom">
                <n-space align="center">
                  <span>自定义规则</span>
                  <n-text depth="3">为此终端配置独立的规则</n-text>
                </n-space>
              </n-radio>
            </n-space>
          </n-radio-group>
        </n-form-item>

        <!-- 显示生效的规则 (非custom模式) -->
        <template v-if="ruleMode !== 'custom' && ruleMode !== 'none' && effectiveRuleSet">
          <n-divider />
          <n-alert type="info" :bordered="false" style="margin-bottom: 16px">
            当前生效规则: {{ effectiveRuleSet.name }} ({{ effectiveRuleSet.approval_mode === 'manual' ? '手动审批' : effectiveRuleSet.approval_mode === 'auto_yes' ? '自动通过' : '智能审批' }})
          </n-alert>
        </template>

        <!-- 自定义规则配置 -->
        <template v-if="ruleMode === 'custom'">
          <n-divider />

          <n-form-item label="规则名称">
            <n-input v-model:value="config.name" placeholder="自定义规则名称" />
          </n-form-item>

          <n-form-item label="审批模式">
            <n-radio-group v-model:value="config.approvalMode">
              <n-space vertical>
                <n-radio value="manual">手动审批</n-radio>
                <n-radio value="auto_yes">自动通过</n-radio>
                <n-radio value="smart">智能审批</n-radio>
              </n-space>
            </n-radio-group>
          </n-form-item>

          <n-form-item label="自动输入类型" v-if="config.approvalMode === 'auto_yes'">
            <n-select v-model:value="config.autoInputType" :options="autoInputOptions" style="width: 200px" />
          </n-form-item>

          <n-divider />

          <n-form-item label="白名单规则">
            <n-dynamic-tags v-model:value="config.whitelistPatterns" />
            <n-text depth="3" style="display: block; margin-top: 8px">匹配这些模式的提示将自动通过</n-text>
          </n-form-item>

          <n-form-item label="黑名单规则">
            <n-dynamic-tags v-model:value="config.blacklistPatterns" />
            <n-text depth="3" style="display: block; margin-top: 8px">匹配这些模式的提示将被阻止</n-text>
          </n-form-item>

          <n-form-item>
            <n-button size="small" @click="loadDefaultPatterns">加载默认规则模板</n-button>
          </n-form-item>

          <n-divider />

          <n-form-item label="AI辅助" v-if="config.approvalMode === 'smart'">
            <n-space vertical style="width: 100%">
              <n-select v-model:value="config.aiProviderId" :options="providerOptions" placeholder="选择AI Provider（可选）" clearable />
              <n-input v-model:value="config.aiPrompt" type="textarea" :rows="3" placeholder="AI判断提示词（可选）" />
            </n-space>
          </n-form-item>

          <n-form-item label="AI代理检测">
            <n-space>
              <n-checkbox v-model:checked="config.detectClaudeCode">Claude Code</n-checkbox>
              <n-checkbox v-model:checked="config.detectCodex">Codex</n-checkbox>
              <n-checkbox v-model:checked="config.detectGemini">Gemini CLI</n-checkbox>
            </n-space>
          </n-form-item>

          <n-form-item label="通知设置">
            <n-space>
              <n-checkbox v-model:checked="config.notifyOnBlock">阻止时通知</n-checkbox>
              <n-checkbox v-model:checked="config.notifyOnApprove">自动通过时通知</n-checkbox>
            </n-space>
          </n-form-item>
        </template>
      </n-form>
    </n-spin>

    <template #footer>
      <n-space justify="end">
        <n-button @click="emit('close')">取消</n-button>
        <n-button type="primary" @click="saveConfig" :loading="saving">保存</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import {
  NModal, NForm, NFormItem, NRadioGroup, NRadio, NSpace, NText, NButton,
  NDivider, NSelect, NInput, NDynamicTags, NCheckbox, NSpin, NAlert, useMessage
} from 'naive-ui'
import { automationApi, type RuleSet } from '@/api'

interface AIProvider {
  id: string
  name: string
  enabled: boolean
}

const props = defineProps<{
  show: boolean
  terminalId: string
  terminalTitle?: string
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const providers = ref<AIProvider[]>([])

const ruleMode = ref('none')
const hasTask = ref(false)
const taskId = ref<string | null>(null)
const currentRuleSetId = ref<string | null>(null)
const effectiveRuleSet = ref<RuleSet | null>(null)

const config = reactive({
  name: '自定义规则',
  approvalMode: 'manual',
  autoInputType: 'yes',
  whitelistPatterns: [] as string[],
  blacklistPatterns: [] as string[],
  aiProviderId: null as string | null,
  aiPrompt: '',
  contextLines: 50,
  detectClaudeCode: true,
  detectCodex: true,
  detectGemini: true,
  notifyOnBlock: true,
  notifyOnApprove: false
})

const autoInputOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'y', value: 'y' },
  { label: 'Enter', value: 'enter' },
  { label: '1 (选项1)', value: 'option1' }
]

const providerOptions = computed(() =>
  providers.value.filter(p => p.enabled).map(p => ({ label: p.name, value: p.id }))
)

async function fetchProviders() {
  try {
    const { data } = await automationApi.listAIProviders()
    providers.value = data.items || []
  } catch (e) {
    console.error('Failed to fetch providers:', e)
  }
}

async function fetchConfig() {
  if (!props.terminalId) return

  loading.value = true
  try {
    const { data } = await automationApi.getTerminalRuleMode(props.terminalId)

    ruleMode.value = data.rule_mode || 'none'
    taskId.value = data.task_id || null
    hasTask.value = !!data.task_id
    currentRuleSetId.value = data.rule_set_id || null
    effectiveRuleSet.value = data.effective_rule_set || null

    // 如果是custom模式且有规则集，加载规则集配置
    if (ruleMode.value === 'custom' && data.rule_set) {
      const rs = data.rule_set
      config.name = rs.name || '自定义规则'
      config.approvalMode = rs.approval_mode || 'manual'
      config.autoInputType = rs.auto_input_type || 'yes'
      config.aiProviderId = rs.ai_provider_id || null
      config.aiPrompt = rs.ai_prompt || ''
      config.contextLines = rs.context_lines || 50
      config.detectClaudeCode = rs.detect_claude_code ?? true
      config.detectCodex = rs.detect_codex ?? true
      config.detectGemini = rs.detect_gemini ?? true
      config.notifyOnBlock = rs.notify_on_block ?? true
      config.notifyOnApprove = rs.notify_on_approve ?? false
      try {
        config.whitelistPatterns = rs.whitelist_patterns ? JSON.parse(rs.whitelist_patterns) : []
        config.blacklistPatterns = rs.blacklist_patterns ? JSON.parse(rs.blacklist_patterns) : []
      } catch {
        config.whitelistPatterns = []
        config.blacklistPatterns = []
      }
    }
  } catch (e) {
    console.error('Failed to fetch terminal config:', e)
  } finally {
    loading.value = false
  }
}

async function loadDefaultPatterns() {
  try {
    const { data } = await automationApi.getDefaultPatterns()
    config.whitelistPatterns = data.whitelist || []
    config.blacklistPatterns = data.blacklist || []
    message.success('已加载默认规则模板')
  } catch (e) {
    message.error('加载失败')
  }
}

async function saveConfig() {
  saving.value = true
  try {
    if (ruleMode.value === 'custom') {
      // 创建或更新自定义规则
      if (currentRuleSetId.value) {
        // 更新现有规则集
        await automationApi.updateRuleSet(currentRuleSetId.value, {
          name: config.name,
          approval_mode: config.approvalMode,
          auto_input_type: config.autoInputType,
          whitelist_patterns: config.whitelistPatterns,
          blacklist_patterns: config.blacklistPatterns,
          ai_provider_id: config.aiProviderId,
          ai_prompt: config.aiPrompt,
          context_lines: config.contextLines,
          detect_claude_code: config.detectClaudeCode,
          detect_codex: config.detectCodex,
          detect_gemini: config.detectGemini,
          notify_on_block: config.notifyOnBlock,
          notify_on_approve: config.notifyOnApprove
        })
      } else {
        // 创建新的自定义规则
        await automationApi.createTerminalCustomRule(props.terminalId, {
          name: config.name,
          approval_mode: config.approvalMode,
          auto_input_type: config.autoInputType,
          whitelist_patterns: config.whitelistPatterns,
          blacklist_patterns: config.blacklistPatterns,
          ai_provider_id: config.aiProviderId,
          ai_prompt: config.aiPrompt,
          context_lines: config.contextLines,
          detect_claude_code: config.detectClaudeCode,
          detect_codex: config.detectCodex,
          detect_gemini: config.detectGemini,
          notify_on_block: config.notifyOnBlock,
          notify_on_approve: config.notifyOnApprove
        })
      }
    } else {
      // 更新规则模式
      await automationApi.updateTerminalRuleMode(props.terminalId, {
        rule_mode: ruleMode.value,
        rule_set_id: null
      })
    }

    message.success('配置已保存')
    emit('saved')
    emit('close')
  } catch (e) {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    fetchProviders()
    fetchConfig()
  }
})
</script>
