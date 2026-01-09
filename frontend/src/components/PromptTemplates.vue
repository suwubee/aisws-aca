<template>
  <n-card size="small" title="AI 提示词模板">
    <template #header-extra>
      <n-button size="small" quaternary :loading="loading" @click="fetchTemplates">刷新</n-button>
    </template>

    <n-spin :show="loading">
      <n-empty v-if="templates.length === 0" description="暂无模板" />

      <n-tabs
        v-else
        v-model:value="activeKey"
        type="segment"
        size="small"
        animated
      >
        <n-tab-pane
          v-for="tpl in templates"
          :key="tpl.key"
          :name="tpl.key"
          :tab="tpl.name || tpl.key"
        >
          <div class="prompt-template">
            <n-space vertical size="12">
              <n-text depth="3">{{ tpl.description || '—' }}</n-text>
              <n-text depth="3" style="font-size: 12px">Key: {{ tpl.key }}</n-text>

              <div class="preset-row">
                <n-space size="10" align="center" wrap>
                  <n-text depth="3" style="font-size: 12px">当前方案：</n-text>
                  <n-select
                    v-model:value="presetSelection[tpl.key]"
                    size="small"
                    style="min-width: 200px"
                    :options="presetOptions(tpl.key)"
                    :loading="presetLoading[tpl.key] === true"
                    placeholder="选择方案"
                  />
                  <n-button
                    size="small"
                    :disabled="!canApplyPreset(tpl)"
                    :loading="applyingPresetKey === tpl.key"
                    @click="applySelectedPreset(tpl)"
                  >
                    套用方案
                  </n-button>
                  <n-button
                    size="small"
                    quaternary
                    :disabled="creatingPresetKey === tpl.key"
                    @click="openCreatePreset(tpl)"
                  >
                    保存为方案
                  </n-button>
                  <n-popconfirm
                    v-if="canDeleteSelectedPreset(tpl.key)"
                    positive-text="删除"
                    negative-text="取消"
                    @positive-click="deleteSelectedPreset(tpl.key)"
                  >
                    <template #trigger>
                      <n-button
                        size="small"
                        quaternary
                        type="error"
                        :loading="deletingPresetKey === tpl.key"
                      >
                        删除方案
                      </n-button>
                    </template>
                    确定删除方案「{{ selectedPresetName(tpl.key) }}」吗？
                  </n-popconfirm>
                  <n-button
                    v-else
                    size="small"
                    quaternary
                    type="error"
                    disabled
                  >
                    删除方案
                  </n-button>
                </n-space>
              </div>

              <div v-if="tpl.variables && tpl.variables.length > 0">
                <n-text depth="3" style="font-size: 12px">可用变量：</n-text>
                <n-space size="6" wrap style="margin-top: 6px">
                  <n-tag
                    v-for="v in tpl.variables"
                    :key="v"
                    size="small"
                    :bordered="false"
                    type="info"
                  >
                    {{ formatVariable(v) }}
                  </n-tag>
                </n-space>
              </div>

              <n-input
                v-model:value="drafts[tpl.key]"
                type="textarea"
                :rows="12"
                placeholder="请输入提示词模板（Go template 语法）"
              />

              <n-space justify="end" size="small" wrap>
                <n-button
                  size="small"
                  :disabled="savingKey === tpl.key"
                  :loading="resettingKey === tpl.key"
                  @click="resetToDefault(tpl.key)"
                >
                  恢复默认
                </n-button>
                <n-button
                  type="primary"
                  size="small"
                  :loading="savingKey === tpl.key"
                  :disabled="!isDirty(tpl.key)"
                  @click="save(tpl.key)"
                >
                  保存
                </n-button>
              </n-space>
            </n-space>
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-spin>
  </n-card>

  <n-modal
    v-model:show="showCreatePreset"
    preset="dialog"
    title="保存为方案"
    style="width: min(520px, 94vw)"
  >
    <n-space vertical size="10">
      <n-text depth="3" style="font-size: 12px">将当前编辑区内容保存为可复用方案（不自动套用）。</n-text>
      <n-form :model="createPresetForm" label-placement="left" label-width="80" size="small">
        <n-form-item label="名称" required>
          <n-input v-model:value="createPresetForm.name" placeholder="例如：更严格/更宽松/适配某团队" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input v-model:value="createPresetForm.description" placeholder="可选" />
        </n-form-item>
      </n-form>
    </n-space>
    <template #action>
      <n-button :disabled="creatingPreset" @click="showCreatePreset = false">取消</n-button>
      <n-button type="primary" :loading="creatingPreset" @click="createPreset">保存</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSpin,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import { promptTemplateApi } from '@/api'

interface PromptTemplateItem {
  key: string
  name: string
  description: string
  template: string
  variables: string[]
  active_preset_id?: string
  updated_at?: string
}

interface PromptTemplatePresetItem {
  id: string
  key: string
  name: string
  description: string
  is_builtin: boolean
}

const message = useMessage()

const loading = ref(false)
const templates = ref<PromptTemplateItem[]>([])
const activeKey = ref<string>('')

const drafts = reactive<Record<string, string>>({})
const originals = reactive<Record<string, string>>({})

const presets = reactive<Record<string, PromptTemplatePresetItem[]>>({})
const presetLoading = reactive<Record<string, boolean>>({})
const presetSelection = reactive<Record<string, string>>({})

const savingKey = ref<string>('')
const resettingKey = ref<string>('')
const applyingPresetKey = ref<string>('')
const deletingPresetKey = ref<string>('')

const showCreatePreset = ref(false)
const creatingPreset = ref(false)
const creatingPresetKey = ref<string>('')
const createPresetForm = reactive({
  key: '',
  name: '',
  description: ''
})

function formatVariable(name: string) {
  const v = String(name || '').trim()
  if (!v) return ''
  return `{{.${v}}}`
}

function isDirty(key: string) {
  const k = String(key || '')
  return (drafts[k] ?? '') !== (originals[k] ?? '')
}

function applyItems(items: PromptTemplateItem[]) {
  templates.value = items
  items.forEach((item) => {
    drafts[item.key] = item.template ?? ''
    originals[item.key] = item.template ?? ''
    presetSelection[item.key] = String(item.active_preset_id || '')
  })
  if (!activeKey.value && items.length > 0) {
    activeKey.value = items[0].key
  }
}

function presetOptions(key: string) {
  const k = String(key || '')
  const items = presets[k] || []
  const options = [{ label: '自定义', value: '' }]
  items.forEach((p) => {
    const label = p.is_builtin ? `${p.name}（内置）` : p.name
    options.push({ label, value: p.id })
  })
  return options
}

function canApplyPreset(tpl: PromptTemplateItem) {
  const selected = String(presetSelection[tpl.key] || '')
  const activePreset = String(tpl.active_preset_id || '')
  return selected !== '' && selected !== activePreset
}

function canDeleteSelectedPreset(key: string) {
  const k = String(key || '')
  const selected = String(presetSelection[k] || '')
  if (!selected) return false
  const items = presets[k] || []
  const found = items.find(p => p.id === selected)
  if (!found) return false
  return !found.is_builtin
}

function selectedPresetName(key: string) {
  const k = String(key || '')
  const selected = String(presetSelection[k] || '')
  if (!selected) return ''
  const items = presets[k] || []
  const found = items.find(p => p.id === selected)
  return found?.name || ''
}

async function fetchPresetsForKey(key: string) {
  const k = String(key || '').trim()
  if (!k) return

  presetLoading[k] = true
  try {
    const { data } = await promptTemplateApi.listPresets(k)
    presets[k] = Array.isArray(data.items) ? data.items : []
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载方案失败')
  } finally {
    presetLoading[k] = false
  }
}

async function fetchTemplates() {
  loading.value = true
  try {
    const { data } = await promptTemplateApi.list()
    const items = Array.isArray(data.items) ? data.items : []
    applyItems(items)
    await Promise.allSettled(items.map(i => fetchPresetsForKey(i.key)))
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载提示词模板失败')
  } finally {
    loading.value = false
  }
}

async function save(key: string) {
  const k = String(key || '').trim()
  if (!k) return

  const text = drafts[k] ?? ''
  if (!String(text).trim()) {
    message.warning('提示词内容不能为空')
    return
  }

  savingKey.value = k
  try {
    const { data } = await promptTemplateApi.update(k, text)
    const item = data.item as PromptTemplateItem
    message.success('已保存')
    originals[k] = item?.template ?? text
    drafts[k] = item?.template ?? text
    templates.value = templates.value.map(t => (t.key === k ? { ...t, ...item } : t))
    presetSelection[k] = String(item?.active_preset_id || '')
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    savingKey.value = ''
  }
}

async function resetToDefault(key: string) {
  const k = String(key || '').trim()
  if (!k) return

  resettingKey.value = k
  try {
    const { data } = await promptTemplateApi.reset(k)
    const item = data.item as PromptTemplateItem
    message.success('已恢复默认')
    originals[k] = item?.template ?? ''
    drafts[k] = item?.template ?? ''
    templates.value = templates.value.map(t => (t.key === k ? { ...t, ...item } : t))
    presetSelection[k] = String(item?.active_preset_id || '')
    await fetchPresetsForKey(k)
  } catch (e: any) {
    message.error(e.response?.data?.error || '恢复默认失败')
  } finally {
    resettingKey.value = ''
  }
}

async function applySelectedPreset(tpl: PromptTemplateItem) {
  const k = String(tpl.key || '').trim()
  if (!k) return
  const id = String(presetSelection[k] || '').trim()
  if (!id) return

  applyingPresetKey.value = k
  try {
    const { data } = await promptTemplateApi.applyPreset(k, id)
    const item = data.item as PromptTemplateItem
    message.success('已套用方案')
    originals[k] = item?.template ?? ''
    drafts[k] = item?.template ?? ''
    templates.value = templates.value.map(t => (t.key === k ? { ...t, ...item } : t))
    presetSelection[k] = String(item?.active_preset_id || '')
  } catch (e: any) {
    message.error(e.response?.data?.error || '套用方案失败')
  } finally {
    applyingPresetKey.value = ''
  }
}

function openCreatePreset(tpl: PromptTemplateItem) {
  const k = String(tpl.key || '').trim()
  if (!k) return
  createPresetForm.key = k
  createPresetForm.name = ''
  createPresetForm.description = ''
  showCreatePreset.value = true
}

async function createPreset() {
  const k = String(createPresetForm.key || '').trim()
  const name = String(createPresetForm.name || '').trim()
  const description = String(createPresetForm.description || '').trim()
  if (!k) return
  if (!name) {
    message.warning('请输入方案名称')
    return
  }

  const text = drafts[k] ?? ''
  if (!String(text).trim()) {
    message.warning('提示词内容不能为空')
    return
  }

  creatingPreset.value = true
  creatingPresetKey.value = k
  try {
    await promptTemplateApi.createPreset(k, { name, description, template: text })
    message.success('方案已保存')
    showCreatePreset.value = false
    await fetchPresetsForKey(k)
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存方案失败')
  } finally {
    creatingPreset.value = false
    creatingPresetKey.value = ''
  }
}

async function deleteSelectedPreset(key: string) {
  const k = String(key || '').trim()
  if (!k) return
  const id = String(presetSelection[k] || '').trim()
  if (!id) return

  const items = presets[k] || []
  const found = items.find(p => p.id === id)
  if (!found || found.is_builtin) {
    message.warning('内置方案不可删除')
    return
  }

  deletingPresetKey.value = k
  try {
    await promptTemplateApi.deletePreset(k, id)
    message.success('方案已删除')
    await fetchPresetsForKey(k)
    // Refresh active template to get latest active_preset_id after delete.
    const { data } = await promptTemplateApi.get(k)
    const item = data.item as PromptTemplateItem
    templates.value = templates.value.map(t => (t.key === k ? { ...t, ...item } : t))
    presetSelection[k] = String(item?.active_preset_id || '')
    originals[k] = item?.template ?? originals[k] ?? ''
    drafts[k] = item?.template ?? drafts[k] ?? ''
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除方案失败')
  } finally {
    deletingPresetKey.value = ''
  }
}

onMounted(() => {
  fetchTemplates()
})
</script>

<style scoped>
.prompt-template {
  padding: 6px 2px;
}

.preset-row {
  padding: 2px 0;
}
</style>
