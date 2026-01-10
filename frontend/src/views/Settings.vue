<template>
  <div class="settings-page">
    <div class="page-header">
      <h2>系统设置</h2>
      <p class="page-desc">配置AI Provider、终端自动化和账户设置</p>
    </div>

    <n-tabs type="line" animated>
      <!-- 账户设置 -->
      <n-tab-pane name="account" tab="账户设置">
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
      </n-tab-pane>

      <!-- 用户管理（仅管理员） -->
      <n-tab-pane v-if="isAdmin" name="users" tab="用户管理">
        <div class="section">
          <div class="section-header">
            <span>用户管理</span>
          </div>
          <UserManagement />
        </div>
      </n-tab-pane>

      <!-- 默认审批规则 -->
      <n-tab-pane name="automation" tab="系统规则">
        <div class="section">
          <div class="section-header">
            <span>系统审批规则配置</span>
            <n-text depth="3">终端可选择使用此系统规则或自定义规则</n-text>
          </div>

          <n-card size="small" style="max-width: 600px">
            <n-form label-placement="left" label-width="120">
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
                  <n-button type="primary" @click="saveSystemConfig" :loading="savingSystemConfig">
                    保存系统规则
                  </n-button>
                  <n-button @click="loadDefaultPatterns">
                    加载默认规则模板
                  </n-button>
                </n-space>
              </n-form-item>
            </n-form>
          </n-card>
        </div>
      </n-tab-pane>

      <!-- 规则集导入导出 -->
      <n-tab-pane name="rule-import-export" tab="规则导入/导出">
        <div class="section">
          <div class="section-header">
            <span>规则集导入 / 导出</span>
            <n-text depth="3">用于备份或迁移规则集配置</n-text>
          </div>
          <RuleImportExport />
        </div>
      </n-tab-pane>

      <!-- 日志导出 -->
      <n-tab-pane name="log-export" tab="日志导出">
        <div class="section">
          <div class="section-header">
            <span>日志导出</span>
            <n-text depth="3">按时间范围导出日志（JSON / CSV），可选按终端ID筛选</n-text>
          </div>
          <LogExport />
        </div>
      </n-tab-pane>

      <!-- AI 代理配置 -->
      <n-tab-pane name="agents" tab="AI代理">
        <div class="section">
          <div class="section-header">
            <span>AI代理配置</span>
            <n-text depth="3">配置各AI代理的检测模式、优先级与启用状态</n-text>
          </div>
          <AgentConfig />
        </div>
      </n-tab-pane>

      <!-- AI Provider 配置 -->
      <n-tab-pane name="ai-providers" tab="AI Provider">
        <div class="section">
          <div class="section-header">
            <span>AI Provider 配置</span>
            <n-button type="primary" size="small" @click="showProviderModal = true">
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
      </n-tab-pane>

      <!-- 按键绑定 -->
      <n-tab-pane name="key-bindings" tab="按键绑定">
        <div class="section">
          <div class="section-header">
            <span>按键绑定</span>
            <n-text depth="3">全局 Enter/换行 等按键配置，终端快捷键与自动化共用</n-text>
          </div>
          <KeyBindings />
        </div>
      </n-tab-pane>

      <!-- 计划任务 -->
      <n-tab-pane name="schedules" tab="计划任务">
        <div class="section">
          <div class="section-header">
            <span>计划任务</span>
            <n-text depth="3">支持 cron / 单次定时，运行任务或 AI 工作流</n-text>
          </div>
          <ScheduledJobs />
        </div>
      </n-tab-pane>

      <!-- 提示词模板 -->
      <n-tab-pane name="prompt-templates" tab="提示词模板">
        <div class="section">
          <div class="section-header">
            <span>提示词模板</span>
            <n-text depth="3">全局 AI 提示词从这里读取，支持变量模板渲染</n-text>
          </div>
          <PromptTemplates />
        </div>
      </n-tab-pane>

      <!-- 消息中心 -->
      <n-tab-pane name="messages" tab="消息中心">
        <div class="section">
          <div class="section-header">
            <span>消息列表</span>
            <n-space>
              <n-select
                v-model:value="messageFilter.status"
                size="small"
                :options="statusOptions"
                style="width: 100px"
                @update:value="fetchMessages"
              />
              <n-button size="small" @click="markAllRead" :disabled="messages.length === 0">
                全部已读
              </n-button>
            </n-space>
          </div>
          <n-data-table
            :columns="messageColumns"
            :data="messages"
            :loading="loadingMessages"
            :row-key="(row: Message) => row.id"
            :pagination="messagePagination"
            size="small"
            @update:page="handleMessagePageChange"
          />
        </div>
      </n-tab-pane>

      <!-- 审批记录 -->
      <n-tab-pane name="approvals" tab="审批记录">
        <div class="section">
          <div class="section-header">
            <span>自动审批记录</span>
            <n-button size="small" quaternary @click="fetchApprovalRecords">
              刷新
            </n-button>
          </div>
          <n-data-table
            :columns="approvalColumns"
            :data="approvalRecords"
            :loading="loadingApprovals"
            :row-key="(row: ApprovalRecord) => row.id"
            :pagination="approvalPagination"
            size="small"
            @update:page="handleApprovalPageChange"
          />
        </div>
      </n-tab-pane>
    </n-tabs>

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
import { ref, h, reactive, onMounted, computed } from 'vue'
import {
  NTabs, NTabPane, NButton, NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSelect, NSwitch, NSlider, NTag, NSpace, NPopconfirm, NCard, NText, NRadioGroup, NRadio,
  NDivider, NDynamicTags, NCheckbox, useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { automationApi, authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useServerStore } from '@/stores/server'
import { useApprovalStore } from '@/stores/approval'
import AgentConfig from '@/components/AgentConfig.vue'
import KeyBindings from '@/components/KeyBindings.vue'
import LogExport from '@/components/LogExport.vue'
import PromptTemplates from '@/components/PromptTemplates.vue'
import RuleImportExport from '@/components/RuleImportExport.vue'
import ScheduledJobs from '@/components/ScheduledJobs.vue'
import UserManagement from '@/components/UserManagement.vue'

// Types
interface AIProvider {
  id: string; name: string; provider: string; base_url: string; model: string
  temperature: number; max_tokens: number; is_default: boolean; enabled: boolean; created_at: string
}

interface Message {
  id: string; terminal_id: string | null; type: string; title: string
  content: string; status: string; priority: number; created_at: string
}

interface ApprovalRecord {
  id: string; terminal_id: string; ai_session_id: string | null; prompt_type: string
  prompt_content: string; response: string; auto_approved?: boolean; auto_handled?: boolean
  rule_matched: string; ai_decision: string; created_at: string
}

const message = useMessage()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()
const serverStore = useServerStore()
const approvalStore = useApprovalStore()
const isAdmin = computed(() => authStore.user?.role === 'admin')

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

const providerColumns: DataTableColumns<AIProvider> = [
  { title: '名称', key: 'name', width: 150 },
  { title: 'Provider', key: 'provider', width: 100, render: (row) => h(NTag, { size: 'small', bordered: false }, () => row.provider) },
  { title: '模型', key: 'model', width: 150 },
  { title: 'Base URL', key: 'base_url', ellipsis: { tooltip: true } },
  { title: '状态', key: 'enabled', width: 80, render: (row) => h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '启用' : '禁用') },
  { title: '默认', key: 'is_default', width: 60, render: (row) => row.is_default ? h(NTag, { size: 'small', type: 'info' }, () => '是') : '-' },
  {
    title: '操作', key: 'actions', width: 120,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => editProvider(row) }, () => '编辑'),
      h(NPopconfirm, { onPositiveClick: () => { void deleteProvider(row.id) } }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, () => '删除'),
        default: () => '确定删除?'
      })
    ])
  }
]

async function fetchProviders() {
  loadingProviders.value = true
  try {
    const { data } = await automationApi.listAIProviders()
    providers.value = data.items || []
  } finally { loadingProviders.value = false }
}

function editProvider(provider: AIProvider) {
  editingProvider.value = provider
  Object.assign(providerForm, { ...provider, api_key: '' })
  showProviderModal.value = true
}

async function saveProvider() {
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

// ===== Messages =====
const messages = ref<Message[]>([])
const loadingMessages = ref(false)
const messageFilter = reactive({ status: null as string | null })
const messagePagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })

const statusOptions = [
  { label: '全部', value: null },
  { label: '未读', value: 'unread' },
  { label: '已读', value: 'read' },
  { label: '已处理', value: 'handled' }
]

const messageColumns: DataTableColumns<Message> = [
  { title: '时间', key: 'created_at', width: 140, render: (row) => new Date(row.created_at).toLocaleString('zh-CN') },
  {
    title: '类型', key: 'type', width: 100,
    render: (row) => {
      const typeMap: Record<string, { label: string; type: 'info' | 'warning' | 'error' }> = {
        approval_needed: { label: '待审批', type: 'warning' },
        blocked: { label: '已阻止', type: 'error' },
        info: { label: '信息', type: 'info' },
        warning: { label: '警告', type: 'warning' },
        error: { label: '错误', type: 'error' }
      }
      const info = typeMap[row.type] || { label: row.type, type: 'info' }
      return h(NTag, { size: 'small', type: info.type }, () => info.label)
    }
  },
  { title: '标题', key: 'title', ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'status', width: 80,
    render: (row) => {
      const statusMap: Record<string, string> = { unread: '未读', read: '已读', handled: '已处理', dismissed: '已忽略' }
      return h(NTag, { size: 'small', bordered: false, type: row.status === 'unread' ? 'warning' : 'default' }, () => statusMap[row.status] || row.status)
    }
  },
  {
    title: '操作', key: 'actions', width: 100,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      row.status === 'unread' && h(NButton, { size: 'tiny', quaternary: true, onClick: () => markMessageRead(row.id) }, () => '已读'),
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => dismissMessage(row.id) }, () => '忽略')
    ].filter(Boolean))
  }
]

async function fetchMessages() {
  loadingMessages.value = true
  try {
    const { data } = await automationApi.listMessages({
      status: messageFilter.status || undefined,
      limit: messagePagination.pageSize,
      offset: (messagePagination.page - 1) * messagePagination.pageSize
    })
    messages.value = data.items || []
    messagePagination.itemCount = data.total || 0
  } finally { loadingMessages.value = false }
}

async function markMessageRead(id: string) {
  try { await automationApi.markMessageRead(id); fetchMessages() } catch { message.error('操作失败') }
}

async function dismissMessage(id: string) {
  try { await automationApi.dismissMessage(id); message.success('已忽略'); fetchMessages() } catch { message.error('操作失败') }
}

async function markAllRead() {
  try { await automationApi.markAllRead(); message.success('全部标记为已读'); fetchMessages() } catch { message.error('操作失败') }
}

function handleMessagePageChange(page: number) { messagePagination.page = page; fetchMessages() }

// ===== Approval Records =====
const approvalRecords = ref<ApprovalRecord[]>([])
const loadingApprovals = ref(false)
const approvalPagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })

function cleanApprovalPrompt(content: string): string {
  if (!content) return ''
  let cleaned = content
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[PX^_].*?\x1b\\/g, '')
    .replace(/\[[0-9;]{1,20}[a-zA-Z]/g, '')
    .replace(/(?:^|\s)(?:[0-9]{1,3};){1,8}[0-9]{1,3}m(?=[+\-\[]|\s)/g, ' ')
    .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
    .trim()
  while (cleaned.includes('\n\n\n')) cleaned = cleaned.replace(/\n\n\n/g, '\n\n')
  return cleaned
}

function normalizeApprovalResponse(response: string) {
  return (response || '').trim()
}

function responseTagType(response: string) {
  const r = normalizeApprovalResponse(response).toLowerCase()
  if (r === 'y' || r === 'yes' || r === 'approve') return 'success'
  if (r === 'n' || r === 'no' || r === 'reject') return 'error'
  return 'default'
}

const approvalColumns: DataTableColumns<ApprovalRecord> = [
  { title: '时间', key: 'created_at', width: 140, render: (row) => new Date(row.created_at).toLocaleString('zh-CN') },
  { title: '类型', key: 'prompt_type', width: 100, render: (row) => h(NTag, { size: 'small', bordered: false }, () => row.prompt_type || 'unknown') },
  { title: '提示内容', key: 'prompt_content', ellipsis: { tooltip: true }, render: (row) => h('pre', { style: { margin: 0, fontSize: '11px', maxHeight: '40px', overflow: 'hidden' } }, cleanApprovalPrompt(row.prompt_content).substring(0, 200)) },
  { title: '响应', key: 'response', width: 80, render: (row) => h(NTag, { size: 'small', bordered: false, type: responseTagType(row.response) }, () => normalizeApprovalResponse(row.response) || '—') },
  { title: '自动处理', key: 'auto_approved', width: 80, render: (row) => (row.auto_approved ?? row.auto_handled) ? h(NTag, { size: 'small', bordered: false, type: 'info' }, () => '是') : '否' },
  { title: '匹配规则', key: 'rule_matched', width: 100, ellipsis: { tooltip: true } }
]

async function fetchApprovalRecords() {
  loadingApprovals.value = true
  try {
    const { data } = await automationApi.listApprovalRecords({
      limit: approvalPagination.pageSize,
      offset: (approvalPagination.page - 1) * approvalPagination.pageSize
    })
    approvalRecords.value = data.items || []
    approvalPagination.itemCount = data.total || 0
  } finally { loadingApprovals.value = false }
}

function handleApprovalPageChange(page: number) { approvalPagination.page = page; fetchApprovalRecords() }

// Init
onMounted(() => {
  fetchProviders()
  fetchSystemConfig()
  fetchMessages()
  fetchApprovalRecords()
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

.section { margin-top: 16px; }
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-weight: 600;
}

:deep(.n-tabs-nav) { background: #252525; padding: 0 16px; border-radius: 6px; }
:deep(.n-data-table) { --n-th-color: #252525; --n-td-color: #1e1e1e; --n-border-color: #333; }
:deep(.n-card) { background: #252525; }
</style>
