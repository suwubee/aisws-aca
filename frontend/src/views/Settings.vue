<template>
  <div class="settings-page" :class="{ 'settings-page--mobile': isMobile }">
    <div class="page-header">
      <h2>系统设置</h2>
      <p class="page-desc">配置AI Provider、终端自动化和账户设置</p>
    </div>

    <div v-if="isMobile" class="settings-mobile-nav">
      <n-select
        v-model:value="activeTab"
        size="small"
        :options="settingsTabOptions"
        placeholder="选择设置项"
        style="width: min(320px, 94vw)"
      />
    </div>

    <div class="settings-layout">
      <div v-if="!isMobile" class="settings-sider">
        <n-menu
          :value="activeTab"
          :options="settingsMenuOptions"
          @update:value="activeTab = $event"
        />
      </div>

      <div class="settings-content">
        <div v-show="activeTab === 'account'">
          <div class="section">
            <div class="section-header">
              <span>修改密码</span>
            </div>
            <n-card size="small" style="max-width: 400px">
              <n-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-placement="left" label-width="100">
                <n-form-item label="当前密码" path="oldPassword">
                  <n-input v-model:value="passwordForm.oldPassword" type="password" show-password-on="click" placeholder="输入当前密码" />
                </n-form-item>
                <n-form-item label="新密码" path="newPassword">
                  <n-input v-model:value="passwordForm.newPassword" type="password" show-password-on="click" placeholder="输入新密码(至少6位)" />
                </n-form-item>
                <n-form-item label="确认密码" path="confirmPassword">
                  <n-input v-model:value="passwordForm.confirmPassword" type="password" show-password-on="click" placeholder="再次输入新密码" />
                </n-form-item>
                <n-form-item>
                  <n-button type="primary" @click="changePassword" :loading="changingPassword">
                    修改密码
                  </n-button>
                </n-form-item>
              </n-form>
            </n-card>
          </div>

          <div v-if="isAdmin" class="section" style="margin-top: 32px">
            <div class="section-header">
              <span>数据管理</span>
            </div>
            <n-card size="small" style="max-width: 400px">
              <n-space vertical>
                <n-text depth="3">重置所有数据将删除除用户与内置模板外的所有数据：任务、终端、日志、审批、消息、服务器/项目/工作流/规则配置等</n-text>
                <n-popconfirm @positive-click="() => { void resetAllData() }" positive-text="确定重置" negative-text="取消">
                  <template #trigger>
                    <n-button type="error" :loading="resettingData">
                      重置所有数据
                    </n-button>
                  </template>
                  <span style="color: #e88080">此操作不可恢复，确定要重置所有数据吗？</span>
                </n-popconfirm>
              </n-space>
            </n-card>
          </div>
        </div>

        <div v-if="isAdmin" v-show="activeTab === 'users'">
          <div class="section">
            <div class="section-header">
              <span>用户管理</span>
            </div>
            <UserManagement />
          </div>
        </div>

        <div v-show="activeTab === 'automation'">
          <div class="section">
            <div class="section-header">
              <span>系统审批规则配置</span>
              <n-text depth="3">终端可选择使用此系统规则或自定义规则</n-text>
            </div>

            <n-card size="small" style="max-width: 600px">
              <n-text v-if="!isAdmin" depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
                当前为只读模式：需要管理员权限才能修改系统规则。
              </n-text>

              <n-form label-placement="left" label-width="120" :disabled="!isAdmin">
                <n-form-item label="审批模式">
                  <n-radio-group v-model:value="defaultAutomation.approvalMode">
                    <n-space vertical>
                      <n-radio value="manual">
                        <n-space align="center">
                          <span>手动审批</span>
                          <n-text depth="3">所有提示都需要手动确认</n-text>
                        </n-space>
                      </n-radio>
                      <n-radio value="auto_yes">
                        <n-space align="center">
                          <span>自动通过</span>
                          <n-text depth="3">自动输入yes/y确认（类似--permission-mode=dontAsk）</n-text>
                        </n-space>
                      </n-radio>
                      <n-radio value="smart">
                        <n-space align="center">
                          <span>智能审批</span>
                          <n-text depth="3">根据规则和AI辅助判断</n-text>
                        </n-space>
                      </n-radio>
                    </n-space>
                  </n-radio-group>
                </n-form-item>

                <n-form-item label="自动输入类型" v-if="defaultAutomation.approvalMode === 'auto_yes'">
                  <n-select v-model:value="defaultAutomation.autoInputType" :options="autoInputOptions" style="width: 200px" />
                </n-form-item>

                <n-divider />

                <n-form-item label="白名单规则">
                  <n-dynamic-tags v-model:value="defaultAutomation.whitelistPatterns" />
                  <n-text depth="3" style="display: block; margin-top: 8px">匹配这些模式的提示将自动通过</n-text>
                </n-form-item>

                <n-form-item label="黑名单规则">
                  <n-dynamic-tags v-model:value="defaultAutomation.blacklistPatterns" />
                  <n-text depth="3" style="display: block; margin-top: 8px">匹配这些模式的提示将被阻止</n-text>
                </n-form-item>

                <n-divider />

                <n-form-item label="AI辅助" v-if="defaultAutomation.approvalMode === 'smart'">
                  <n-space vertical style="width: 100%">
                    <n-select v-model:value="defaultAutomation.aiProviderId" :options="providerOptions" placeholder="选择AI Provider（可选）" clearable />
                    <n-input v-model:value="defaultAutomation.aiPrompt" type="textarea" :rows="3" placeholder="AI判断提示词（可选）" />
                    <n-text depth="3" style="font-size: 12px">
                      提示：此处为规则集的 AI 规则补充（会注入到全局审批系统提示词的变量 <n-text code>extra_rules</n-text>）。
                      全局系统提示词请在「提示词模板」中配置。
                    </n-text>
                  </n-space>
                </n-form-item>

                <n-form-item label="AI代理检测">
                  <n-space>
                    <n-checkbox v-model:checked="defaultAutomation.detectClaudeCode">Claude Code</n-checkbox>
                    <n-checkbox v-model:checked="defaultAutomation.detectCodex">Codex</n-checkbox>
                    <n-checkbox v-model:checked="defaultAutomation.detectGemini">Gemini CLI</n-checkbox>
                  </n-space>
                </n-form-item>

                <n-form-item label="通知设置">
                  <n-space>
                    <n-checkbox v-model:checked="defaultAutomation.notifyOnBlock">阻止时通知</n-checkbox>
                    <n-checkbox v-model:checked="defaultAutomation.notifyOnApprove">自动通过时通知</n-checkbox>
                  </n-space>
                </n-form-item>

                <n-form-item>
                  <n-space>
                    <n-button type="primary" @click="saveSystemConfig" :loading="savingSystemConfig" :disabled="!isAdmin">
                      保存系统规则
                    </n-button>
                    <n-button @click="loadDefaultPatterns" :disabled="!isAdmin">
                      加载默认规则模板
                    </n-button>
                  </n-space>
                </n-form-item>
              </n-form>
            </n-card>
          </div>
        </div>

        <div v-show="activeTab === 'rule-import-export'">
          <div class="section">
            <div class="section-header">
              <span>规则集导入 / 导出</span>
              <n-text depth="3">用于备份或迁移规则集配置</n-text>
            </div>
            <RuleImportExport :readonly="!isAdmin" />
          </div>
        </div>

        <div v-show="activeTab === 'log-export'">
          <div class="section">
            <div class="section-header">
              <span>日志导出</span>
              <n-text depth="3">按时间范围导出日志（JSON / CSV），可选按终端ID筛选</n-text>
            </div>
            <LogExport />
          </div>
        </div>

        <div v-show="activeTab === 'agents'">
          <div class="section">
            <div class="section-header">
              <span>AI代理配置</span>
              <n-text depth="3">配置各AI代理的检测模式、优先级与启用状态</n-text>
            </div>
            <AgentConfig />
          </div>
        </div>

        <div v-show="activeTab === 'ai-providers'">
          <div class="section">
            <div class="section-header">
              <span>AI Provider 配置</span>
              <n-button v-if="isAdmin" type="primary" size="small" @click="showProviderModal = true">
                添加 Provider
              </n-button>
            </div>
            <n-data-table
              :columns="providerColumns"
              :data="providers"
              :loading="loadingProviders"
              :row-key="(row: AIProvider) => row.id"
              size="small"
            />
          </div>
        </div>

        <div v-show="activeTab === 'key-bindings'">
          <div class="section">
            <div class="section-header">
              <span>按键绑定</span>
              <n-text depth="3">全局 Enter/换行 等按键配置，终端快捷键与自动化共用</n-text>
            </div>
            <KeyBindings />
          </div>
        </div>

        <div v-show="activeTab === 'prompt-templates'">
          <div class="section">
            <div class="section-header">
              <span>提示词模板</span>
              <n-text depth="3">全局 AI 提示词从这里读取，支持变量模板渲染</n-text>
            </div>
            <PromptTemplates />
          </div>
        </div>
      </div>
    </div>

    <!-- 添加/编辑 Provider Modal -->
    <n-modal
      v-model:show="showProviderModal"
      preset="dialog"
      :title="editingProvider ? '编辑 AI Provider' : '添加 AI Provider'"
      positive-text="保存"
      negative-text="取消"
      style="width: min(500px, 94vw)"
      @positive-click="saveProvider"
    >
      <n-form ref="providerFormRef" :model="providerForm" :rules="providerRules" label-placement="left" label-width="100">
        <n-form-item label="名称" path="name">
          <n-input v-model:value="providerForm.name" placeholder="例如: DeepSeek-Chat" />
        </n-form-item>
        <n-form-item label="Provider" path="provider">
          <n-select v-model:value="providerForm.provider" :options="providerTypeOptions" placeholder="选择 Provider 类型" />
        </n-form-item>
        <n-form-item label="Base URL" path="base_url">
          <n-input v-model:value="providerForm.base_url" placeholder="例如: https://api.deepseek.com/v1" />
        </n-form-item>
        <n-form-item label="API Key" path="api_key">
          <n-input v-model:value="providerForm.api_key" type="password" show-password-on="click" placeholder="sk-..." />
        </n-form-item>
        <n-form-item label="模型" path="model">
          <n-input v-model:value="providerForm.model" placeholder="例如: deepseek-chat" />
        </n-form-item>
        <n-form-item label="温度" path="temperature">
          <n-space>
            <n-slider v-model:value="providerForm.temperature" :min="0" :max="2" :step="0.1" style="width: 200px" />
            <n-input-number v-model:value="providerForm.temperature" :min="0" :max="2" :step="0.1" style="width: 80px" />
          </n-space>
        </n-form-item>
        <n-form-item label="Max Tokens" path="max_tokens">
          <n-input-number v-model:value="providerForm.max_tokens" :min="100" :max="32000" />
        </n-form-item>
        <n-form-item label="启用" path="enabled">
          <n-switch v-model:value="providerForm.enabled" />
        </n-form-item>
        <n-form-item label="设为默认" path="is_default">
          <n-switch v-model:value="providerForm.is_default" />
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, h, reactive, onMounted, computed, watch } from 'vue'
import {
  NButton, NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSelect, NSwitch, NSlider, NTag, NSpace, NPopconfirm, NCard, NText, NRadioGroup, NRadio,
  NDivider, NDynamicTags, NCheckbox, NMenu, useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules, MenuOption } from 'naive-ui'
import { automationApi, authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useServerStore } from '@/stores/server'
import { useApprovalStore } from '@/stores/approval'
import { useIsMobile } from '@/utils/useIsMobile'
import AgentConfig from '@/components/AgentConfig.vue'
import KeyBindings from '@/components/KeyBindings.vue'
import LogExport from '@/components/LogExport.vue'
import PromptTemplates from '@/components/PromptTemplates.vue'
import RuleImportExport from '@/components/RuleImportExport.vue'
import UserManagement from '@/components/UserManagement.vue'

// Types
interface AIProvider {
  id: string; name: string; provider: string; base_url: string; model: string
  temperature: number; max_tokens: number; is_default: boolean; enabled: boolean; created_at: string
}

const message = useMessage()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()
const serverStore = useServerStore()
const approvalStore = useApprovalStore()
const isAdmin = computed(() => authStore.user?.role === 'admin')
const { isMobile } = useIsMobile()

const activeTab = ref('account')

const readOnlyTabs = computed(() => new Set([
  'automation',
  'prompt-templates',
  'key-bindings',
  'rule-import-export',
  'agents',
  'ai-providers'
]))

function maybeReadOnlyLabel(label: string, key: string) {
  if (!isAdmin.value) {
    if (readOnlyTabs.value.has(key)) return `${label}（只读）`
  }
  return label
}

const settingsTabOptions = computed(() => {
  const options: Array<{ label: string; value: string }> = [
    { label: '账户设置', value: 'account' }
  ]

  if (isAdmin.value) {
    options.push({ label: '用户管理', value: 'users' })
  }

  options.push(
    { label: maybeReadOnlyLabel('系统规则', 'automation'), value: 'automation' },
    { label: maybeReadOnlyLabel('提示词模板', 'prompt-templates'), value: 'prompt-templates' },
    { label: maybeReadOnlyLabel('按键绑定', 'key-bindings'), value: 'key-bindings' },
    { label: maybeReadOnlyLabel('规则导入/导出', 'rule-import-export'), value: 'rule-import-export' },
    { label: '日志导出', value: 'log-export' },
    { label: maybeReadOnlyLabel('AI代理', 'agents'), value: 'agents' },
    { label: maybeReadOnlyLabel('AI Provider', 'ai-providers'), value: 'ai-providers' }
  )

  return options
})

const settingsMenuOptions = computed<MenuOption[]>(() => {
  const accountChildren: MenuOption[] = [
    { label: '账户设置', key: 'account', icon: () => h('span', { style: 'font-size: 18px' }, '👤') }
  ]
  if (isAdmin.value) {
    accountChildren.push({ label: '用户管理', key: 'users', icon: () => h('span', { style: 'font-size: 18px' }, '👥') })
  }

  return [
    {
      type: 'group',
      label: '账号',
      key: 'g-account',
      children: accountChildren
    },
    {
      type: 'group',
      label: '治理',
      key: 'g-govern',
      children: [
        { label: maybeReadOnlyLabel('系统规则', 'automation'), key: 'automation', icon: () => h('span', { style: 'font-size: 18px' }, '✅') },
        { label: maybeReadOnlyLabel('提示词模板', 'prompt-templates'), key: 'prompt-templates', icon: () => h('span', { style: 'font-size: 18px' }, '🧩') },
        { label: maybeReadOnlyLabel('按键绑定', 'key-bindings'), key: 'key-bindings', icon: () => h('span', { style: 'font-size: 18px' }, '⌨️') }
      ]
    },
    {
      type: 'group',
      label: 'AI',
      key: 'g-ai',
      children: [
        { label: maybeReadOnlyLabel('AI代理', 'agents'), key: 'agents', icon: () => h('span', { style: 'font-size: 18px' }, '🤖') },
        { label: maybeReadOnlyLabel('AI Provider', 'ai-providers'), key: 'ai-providers', icon: () => h('span', { style: 'font-size: 18px' }, '🧠') }
      ]
    },
    {
      type: 'group',
      label: '备份与导出',
      key: 'g-export',
      children: [
        { label: maybeReadOnlyLabel('规则导入/导出', 'rule-import-export'), key: 'rule-import-export', icon: () => h('span', { style: 'font-size: 18px' }, '📦') },
        { label: '日志导出', key: 'log-export', icon: () => h('span', { style: 'font-size: 18px' }, '📤') }
      ]
    }
  ]
})

watch(isAdmin, (admin) => {
  if (!admin && activeTab.value === 'users') {
    activeTab.value = 'account'
  }
})

// ===== Account Settings =====
const passwordFormRef = ref<FormInst | null>(null)
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const changingPassword = ref(false)
const resettingData = ref(false)

const passwordRules: FormRules = {
  oldPassword: { required: true, message: '请输入当前密码' },
  newPassword: [
    { required: true, message: '请输入新密码' },
    { min: 6, message: '密码至少6位' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码' },
    {
      validator: (_rule, value) => value === passwordForm.newPassword,
      message: '两次输入的密码不一致'
    }
  ]
}

async function changePassword() {
  try {
    await passwordFormRef.value?.validate()
  } catch { return }

  changingPassword.value = true
  try {
    await authApi.changePassword(passwordForm.oldPassword, passwordForm.newPassword)
    message.success('密码修改成功')
    Object.assign(passwordForm, { oldPassword: '', newPassword: '', confirmPassword: '' })
  } catch (e: any) {
    message.error(e.response?.data?.error || '修改密码失败')
  } finally {
    changingPassword.value = false
  }
}

async function resetAllData() {
  resettingData.value = true
  try {
    const { data } = await authApi.resetData()
    message.success('所有数据已重置')
    if (data?.warning) {
      message.warning(data.warning)
    }

    terminalStore.terminals.forEach(t => t.ws?.close())
    terminalStore.terminals = []
    terminalStore.activeTerminalId = null
    approvalStore.pendingApprovals = []
    serverStore.servers = []
    serverStore.loaded = false
    await Promise.allSettled([
      taskStore.fetchTasks(),
      terminalStore.fetchTerminals(),
      serverStore.fetchServers({ force: true })
    ])
  } catch (e) {
    message.error('重置失败')
  } finally {
    resettingData.value = false
  }
}

// ===== Automation Settings =====
const defaultAutomation = reactive({
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
const savingSystemConfig = ref(false)

const autoInputOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'y', value: 'y' },
  { label: 'no', value: 'no' },
  { label: 'n', value: 'n' },
  { label: 'Enter', value: 'enter' },
  { label: '1 (选项1)', value: 'option1' },
  { label: '2 (选项2)', value: 'option2' }
]

const providerOptions = computed(() =>
  providers.value.filter(p => p.enabled).map(p => ({ label: p.name, value: p.id }))
)

async function fetchSystemConfig() {
  try {
    const { data } = await automationApi.getSystemRule()
    const item = data.item
    if (!item) return
    defaultAutomation.approvalMode = item.approval_mode || 'manual'
    defaultAutomation.autoInputType = item.auto_input_type || 'yes'
    defaultAutomation.aiProviderId = item.ai_provider_id || null
    defaultAutomation.aiPrompt = item.ai_prompt || ''
    defaultAutomation.contextLines = item.context_lines || 50
    defaultAutomation.detectClaudeCode = item.detect_claude_code ?? true
    defaultAutomation.detectCodex = item.detect_codex ?? true
    defaultAutomation.detectGemini = item.detect_gemini ?? true
    defaultAutomation.notifyOnBlock = item.notify_on_block ?? true
    defaultAutomation.notifyOnApprove = item.notify_on_approve ?? false
    // 解析JSON数组
    try {
      defaultAutomation.whitelistPatterns = item.whitelist_patterns ? JSON.parse(item.whitelist_patterns) : []
      defaultAutomation.blacklistPatterns = item.blacklist_patterns ? JSON.parse(item.blacklist_patterns) : []
    } catch {
      defaultAutomation.whitelistPatterns = []
      defaultAutomation.blacklistPatterns = []
    }
  } catch (e) {
    console.error('Failed to fetch system config:', e)
  }
}

async function saveSystemConfig() {
  savingSystemConfig.value = true
  try {
    await automationApi.updateSystemRule({
      approval_mode: defaultAutomation.approvalMode,
      auto_input_type: defaultAutomation.autoInputType,
      whitelist_patterns: defaultAutomation.whitelistPatterns,
      blacklist_patterns: defaultAutomation.blacklistPatterns,
      ai_provider_id: defaultAutomation.aiProviderId,
      ai_prompt: defaultAutomation.aiPrompt,
      context_lines: defaultAutomation.contextLines,
      detect_claude_code: defaultAutomation.detectClaudeCode,
      detect_codex: defaultAutomation.detectCodex,
      detect_gemini: defaultAutomation.detectGemini,
      notify_on_block: defaultAutomation.notifyOnBlock,
      notify_on_approve: defaultAutomation.notifyOnApprove
    })
    message.success('系统规则已保存')
  } catch (e) {
    message.error('保存失败')
  } finally {
    savingSystemConfig.value = false
  }
}

async function loadDefaultPatterns() {
  try {
    const { data } = await automationApi.getDefaultPatterns()
    defaultAutomation.whitelistPatterns = data.whitelist || []
    defaultAutomation.blacklistPatterns = data.blacklist || []
    message.success('已加载系统默认规则')
  } catch (e) {
    message.error('加载失败')
  }
}

// ===== AI Providers =====
const providers = ref<AIProvider[]>([])
const loadingProviders = ref(false)
const showProviderModal = ref(false)
const editingProvider = ref<AIProvider | null>(null)
const providerFormRef = ref<FormInst | null>(null)

const providerForm = reactive({
  name: '', provider: 'openai', base_url: '', api_key: '', model: '',
  temperature: 0.7, max_tokens: 2048, enabled: true, is_default: false
})

const providerRules: FormRules = {
  name: { required: true, message: '请输入名称' },
  provider: { required: true, message: '请选择 Provider 类型' },
  model: { required: true, message: '请输入模型名称' }
}

const providerTypeOptions = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'DeepSeek', value: 'deepseek' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Ollama', value: 'ollama' },
  { label: '其他兼容', value: 'custom' }
]

const providerColumns = computed<DataTableColumns<AIProvider>>(() => {
  const cols: DataTableColumns<AIProvider> = [
  { title: '名称', key: 'name', width: 150 },
  { title: 'Provider', key: 'provider', width: 100, render: (row) => h(NTag, { size: 'small', bordered: false }, () => row.provider) },
  { title: '模型', key: 'model', width: 150 },
  { title: 'Base URL', key: 'base_url', ellipsis: { tooltip: true } },
  { title: '状态', key: 'enabled', width: 80, render: (row) => h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '启用' : '禁用') },
  { title: '默认', key: 'is_default', width: 60, render: (row) => row.is_default ? h(NTag, { size: 'small', type: 'info' }, () => '是') : '-' },
  ]

  if (!isAdmin.value) return cols

  cols.push({
    title: '操作', key: 'actions', width: 120,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => editProvider(row) }, () => '编辑'),
      h(NPopconfirm, { onPositiveClick: () => { void deleteProvider(row.id) } }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, () => '删除'),
        default: () => '确定删除?'
      })
    ])
  })

  return cols
})

async function fetchProviders() {
  loadingProviders.value = true
  try {
    const { data } = await automationApi.listAIProviders()
    providers.value = data.items || []
  } finally { loadingProviders.value = false }
}

function editProvider(provider: AIProvider) {
  if (!isAdmin.value) return
  editingProvider.value = provider
  Object.assign(providerForm, { ...provider, api_key: '' })
  showProviderModal.value = true
}

async function saveProvider() {
  if (!isAdmin.value) return false
  try { await providerFormRef.value?.validate() } catch { return false }
  try {
    if (editingProvider.value) {
      await automationApi.updateAIProvider(editingProvider.value.id, providerForm)
      message.success('更新成功')
    } else {
      await automationApi.createAIProvider(providerForm)
      message.success('添加成功')
    }
    showProviderModal.value = false
    editingProvider.value = null
    resetProviderForm()
    fetchProviders()
  } catch { message.error('保存失败'); return false }
}

async function deleteProvider(id: string) {
  if (!isAdmin.value) return
  try {
    await automationApi.deleteAIProvider(id)
    message.success('删除成功')
    fetchProviders()
  } catch { message.error('删除失败') }
}

function resetProviderForm() {
  Object.assign(providerForm, {
    name: '', provider: 'openai', base_url: '', api_key: '', model: '',
    temperature: 0.7, max_tokens: 2048, enabled: true, is_default: false
  })
}

// Init
onMounted(() => {
  fetchProviders()
  fetchSystemConfig()
})
</script>

<style scoped>
.settings-page {
  padding: 20px 24px;
  background: #1a1a1a;
  min-height: calc(100vh - var(--app-header-height) - var(--app-bottom-nav-height));
  color: #e0e0e0;
}

@media (max-width: 768px) {
  .settings-page {
    padding: 12px;
  }
}

.page-header { margin-bottom: 24px; }
.page-header h2 { margin: 0 0 4px 0; font-size: 20px; font-weight: 600; }
.page-desc { margin: 0; font-size: 13px; color: #888; }

.settings-mobile-nav {
  margin-bottom: 12px;
}

.settings-layout {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.settings-sider {
  width: 220px;
  flex-shrink: 0;
}

.settings-content {
  flex: 1;
  min-width: 0;
}

.section { margin-top: 16px; }
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-weight: 600;
}

:deep(.n-menu) { background: #252525; border-radius: 6px; padding: 8px 6px; }
:deep(.n-data-table) { --n-th-color: #252525; --n-td-color: #1e1e1e; --n-border-color: #333; }
:deep(.n-card) { background: #252525; }

@media (max-width: 768px) {
  .settings-layout {
    display: block;
  }

  .settings-sider {
    display: none;
  }
}
</style>
