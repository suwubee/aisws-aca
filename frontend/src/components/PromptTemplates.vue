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
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NEmpty, NInput, NSpace, NSpin, NTabPane, NTabs, NTag, NText, useMessage } from 'naive-ui'
import { promptTemplateApi } from '@/api'

interface PromptTemplateItem {
  key: string
  name: string
  description: string
  template: string
  variables: string[]
  updated_at?: string
}

const message = useMessage()

const loading = ref(false)
const templates = ref<PromptTemplateItem[]>([])
const activeKey = ref<string>('')

const drafts = reactive<Record<string, string>>({})
const originals = reactive<Record<string, string>>({})

const savingKey = ref<string>('')
const resettingKey = ref<string>('')

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
  })
  if (!activeKey.value && items.length > 0) {
    activeKey.value = items[0].key
  }
}

async function fetchTemplates() {
  loading.value = true
  try {
    const { data } = await promptTemplateApi.list()
    const items = Array.isArray(data.items) ? data.items : []
    applyItems(items)
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
  } catch (e: any) {
    message.error(e.response?.data?.error || '恢复默认失败')
  } finally {
    resettingKey.value = ''
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
</style>

