<template>
  <n-card title="AI 代理配置" size="small">
    <div class="agent-config">
      <div v-if="loading" class="loading">
        加载中...
      </div>

      <n-list v-else bordered>
        <n-list-item v-for="agent in configs" :key="agent.agent_type">
          <div class="agent-item">
            <div class="agent-header">
              <div class="agent-meta">
                <div class="agent-name">{{ agent.display_name }}</div>
                <div class="agent-type">{{ agent.agent_type }}</div>
              </div>

              <div class="agent-controls">
                <div class="control">
                  <span class="label">启用</span>
                  <n-switch v-model:value="agent.enabled" />
                </div>
                <div class="control">
                  <span class="label">优先级</span>
                  <n-input
                    v-model:value="agent.priorityInput"
                    size="small"
                    placeholder="0"
                    style="width: 96px"
                    @blur="normalizePriority(agent)"
                  />
                </div>
              </div>
            </div>

            <div class="modes">
              <div class="modes-title">检测模式</div>

              <div class="mode-add">
                <n-input
                  v-model:value="newMode[agent.agent_type]"
                  size="small"
                  placeholder="添加自定义检测模式（正则/关键词）"
                />
                <n-button size="small" @click="addMode(agent.agent_type)">添加</n-button>
              </div>

              <n-list size="small" bordered class="mode-list">
                <template v-if="agent.detect_modes.length === 0">
                  <n-list-item>
                    <div class="mode-empty">暂无检测模式</div>
                  </n-list-item>
                </template>
                <template v-else>
                  <n-list-item
                    v-for="(mode, idx) in agent.detect_modes"
                    :key="`${agent.agent_type}:${idx}`"
                  >
                    <div class="mode-item">
                      <span class="mode-text">{{ mode }}</span>
                      <n-button size="tiny" quaternary type="error" @click="removeMode(agent.agent_type, idx)">
                        删除
                      </n-button>
                    </div>
                  </n-list-item>
                </template>
              </n-list>
            </div>
          </div>
        </n-list-item>
      </n-list>

      <div class="actions">
        <n-button type="primary" :loading="saving" @click="saveConfigs">
          保存配置
        </n-button>
        <n-button :disabled="saving" @click="resetDefaults">
          恢复默认
        </n-button>
      </div>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { NCard, NSwitch, NInput, NButton, NList, NListItem, useMessage } from 'naive-ui'
import { automationApi, type AgentConfig, type AIAgentType } from '@/api'

type AgentConfigForm = AgentConfig & { priorityInput: string }

const message = useMessage()
const loading = ref(false)
const saving = ref(false)

const DEFAULT_AGENTS: AgentConfig[] = [
  {
    agent_type: 'claude-code',
    display_name: 'Claude Code',
    enabled: true,
    priority: 100,
    detect_modes: [
      '(?i)claude\\s*code',
      '(?i)anthropic',
      '(?i)╭─.*claude',
      '(?i)claude.*>',
      '(?i)\\[claude\\]'
    ]
  },
  {
    agent_type: 'codex',
    display_name: 'OpenAI Codex',
    enabled: true,
    priority: 90,
    detect_modes: [
      '(?i)codex',
      '(?i)openai.*cli',
      '(?i)\\[codex\\]'
    ]
  },
  {
    agent_type: 'gemini',
    display_name: 'Gemini CLI',
    enabled: true,
    priority: 80,
    detect_modes: [
      '(?i)gemini',
      '(?i)google\\s*ai',
      '(?i)\\[gemini\\]'
    ]
  },
  {
    agent_type: 'copilot',
    display_name: 'GitHub Copilot',
    enabled: true,
    priority: 70,
    detect_modes: [
      '(?i)github\\s*copilot',
      '(?i)copilot\\s*cli',
      '(?i)\\[copilot\\]'
    ]
  },
  {
    agent_type: 'cursor',
    display_name: 'Cursor',
    enabled: true,
    priority: 60,
    detect_modes: [
      '(?i)cursor\\s*ai',
      '(?i)\\[cursor\\]'
    ]
  }
]

const configs = ref<AgentConfigForm[]>(toForm(DEFAULT_AGENTS))
const newMode = reactive<Record<AIAgentType, string>>({
  'claude-code': '',
  codex: '',
  gemini: '',
  copilot: '',
  cursor: ''
})

function toForm(items: AgentConfig[]): AgentConfigForm[] {
  return items.map((item) => ({
    ...item,
    priorityInput: String(item.priority ?? 0),
    detect_modes: Array.isArray(item.detect_modes) ? [...item.detect_modes] : []
  }))
}

function uniqueNonEmptyModes(modes: string[]) {
  const seen = new Set<string>()
  const result: string[] = []
  for (const mode of modes) {
    const trimmed = (mode || '').trim()
    if (!trimmed) continue
    if (seen.has(trimmed)) continue
    seen.add(trimmed)
    result.push(trimmed)
  }
  return result
}

function normalizePriority(agent: AgentConfigForm) {
  const parsed = Number.parseInt((agent.priorityInput || '').trim(), 10)
  if (Number.isNaN(parsed)) {
    agent.priorityInput = String(agent.priority ?? 0)
    return
  }
  agent.priority = parsed
  agent.priorityInput = String(parsed)
}

function addMode(agentType: AIAgentType) {
  const agent = configs.value.find(a => a.agent_type === agentType)
  if (!agent) return

  const mode = (newMode[agentType] || '').trim()
  if (!mode) return

  const current = uniqueNonEmptyModes(agent.detect_modes)
  if (current.includes(mode)) {
    message.warning('该检测模式已存在')
    return
  }
  agent.detect_modes = [...current, mode]
  newMode[agentType] = ''
}

function removeMode(agentType: AIAgentType, index: number) {
  const agent = configs.value.find(a => a.agent_type === agentType)
  if (!agent) return
  if (index < 0 || index >= agent.detect_modes.length) return
  agent.detect_modes.splice(index, 1)
}

function resetDefaults() {
  configs.value = toForm(DEFAULT_AGENTS)
  for (const key of Object.keys(newMode) as AIAgentType[]) {
    newMode[key] = ''
  }
  message.success('已恢复默认配置')
}

async function fetchConfigs() {
  loading.value = true
  try {
    const { data } = await automationApi.getAgentConfigs()
    const items = Array.isArray(data.items) ? (data.items as AgentConfig[]) : []

    const serverMap = new Map<AIAgentType, AgentConfig>()
    for (const item of items) {
      if (!item?.agent_type) continue
      serverMap.set(item.agent_type, item)
    }

    const merged = DEFAULT_AGENTS.map((def) => {
      const server = serverMap.get(def.agent_type)
      if (!server) return def
      return {
        ...def,
        ...server,
        agent_type: def.agent_type,
        display_name: server.display_name || def.display_name,
        enabled: server.enabled ?? def.enabled,
        priority: typeof server.priority === 'number' ? server.priority : def.priority,
        detect_modes: Array.isArray(server.detect_modes) ? server.detect_modes : def.detect_modes
      }
    })

    configs.value = toForm(merged)
  } catch (e: any) {
    message.error(e?.response?.data?.error || '加载代理配置失败')
    configs.value = toForm(DEFAULT_AGENTS)
  } finally {
    loading.value = false
  }
}

async function saveConfigs() {
  saving.value = true
  try {
    const payload: AgentConfig[] = configs.value.map((agent) => {
      const priority = Number.parseInt((agent.priorityInput || '').trim(), 10)
      return {
        agent_type: agent.agent_type,
        display_name: agent.display_name,
        enabled: agent.enabled,
        priority: Number.isNaN(priority) ? (agent.priority ?? 0) : priority,
        detect_modes: uniqueNonEmptyModes(agent.detect_modes)
      }
    })

    await automationApi.updateAgentConfigs(payload)
    message.success('代理配置已保存')
    await fetchConfigs()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchConfigs()
})
</script>

<style scoped>
.agent-config {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.loading {
  padding: 12px;
  color: #888;
  font-size: 13px;
}

.agent-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.agent-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.agent-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.agent-name {
  font-weight: 600;
  font-size: 14px;
}

.agent-type {
  font-size: 12px;
  color: #888;
}

.agent-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

.control {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label {
  font-size: 12px;
  color: #aaa;
}

.modes {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.modes-title {
  font-size: 12px;
  color: #aaa;
}

.mode-add {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mode-list :deep(.n-list-item) {
  padding: 8px 12px;
}

.mode-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mode-text {
  font-size: 12px;
  color: #ddd;
  word-break: break-all;
}

.mode-empty {
  font-size: 12px;
  color: #888;
}

.actions {
  display: flex;
  gap: 8px;
}
</style>
