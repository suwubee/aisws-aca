<template>
  <n-card
    size="small"
    title="AI 托管会话"
    class="workflow-card"
    :class="{ 'workflow-card--external-scroll': props.externalScroll }"
  >
    <template #header-extra>
      <n-space align="center" size="small">
        <n-tag v-if="session" size="small" :bordered="false" :type="statusTagType(session.status)">
          {{ session.status || 'unknown' }}
        </n-tag>
        <n-button
          v-if="canToggleContext"
          size="tiny"
          quaternary
          :disabled="!session || !hasSessionContext"
          @click="toggleContextCollapsed"
        >
          {{ contextCollapsed ? '展开上下文' : '收起上下文' }}
        </n-button>
        <n-button
          v-if="props.panelMode !== 'flow'"
          size="tiny"
          type="warning"
          quaternary
          :loading="pausing"
          :disabled="!canPauseSession"
          @click="pauseSession"
        >
          暂停
        </n-button>
        <n-button size="tiny" quaternary :disabled="!sessionId || loading" @click="refresh(false)">刷新</n-button>
      </n-space>
    </template>

    <n-spin :show="loading" class="workflow-spin">
      <div class="workflow-body">
        <n-empty v-if="!sessionId" description="暂无会话" />
        <n-empty v-else-if="!session" description="加载中..." />

        <template v-else>
          <div
            v-if="showContextSection"
            class="session-context"
            :class="{ 'session-context--collapsed': contextCollapsed }"
          >
            <div class="session-head">
              <div class="session-head__top">
                <n-text depth="3" class="mono session-head__id">{{ session.id }}</n-text>
                <n-tag size="small" :bordered="false" :type="statusTagType(session.status)">
                  {{ session.status || 'unknown' }}
                </n-tag>
              </div>
              <n-text depth="3" class="session-head__meta">
                {{ formatDateTime(session.started_at) }}
                <span v-if="session.completed_at"> → {{ formatDateTime(session.completed_at) }}</span>
              </n-text>
              <n-text v-if="sessionTerminalID" depth="3" class="session-head__meta">
                terminal: <span class="mono">{{ sessionTerminalID }}</span>
              </n-text>
              <n-text v-if="sessionGoalText" depth="2" class="session-head__goal">
                {{ collapsedText('session:user_goal', sessionGoalText, 320) }}
              </n-text>
              <n-button
                v-if="shouldCollapseText(sessionGoalText, 320)"
                class="payload-toggle"
                size="tiny"
                text
                @click="toggleExpanded('session:user_goal')"
              >
                {{ isExpanded('session:user_goal') ? '收起目标' : '展开目标' }}
              </n-button>
            </div>

            <n-alert
              v-if="sessionSummaryText"
              :bordered="false"
              :type="safeText(session.status).toLowerCase() === 'completed' ? 'success' : 'warning'"
            >
              <pre class="session-summary-pre">{{ collapsedText('session:summary', sessionSummaryText, 520) }}</pre>
              <n-button
                v-if="shouldCollapseText(sessionSummaryText, 520)"
                class="payload-toggle"
                size="tiny"
                text
                @click="toggleExpanded('session:summary')"
              >
                {{ isExpanded('session:summary') ? '收起摘要' : '展开摘要' }}
              </n-button>
            </n-alert>

            <div v-if="canResume(session.status)" class="resume-panel">
              <n-input
                v-model:value="resumeMessage"
                type="textarea"
                :autosize="{ minRows: 2, maxRows: 4 }"
                :placeholder="resumePlaceholder(session.status)"
                :disabled="isDemoMode"
                @keydown.ctrl.enter.prevent="resume"
              />
              <n-space justify="end" style="margin-top: 8px">
                <n-button
                  size="small"
                  type="primary"
                  :loading="resuming"
                  :disabled="isDemoMode || !resumeMessage.trim()"
                  @click="resume"
                >
                  {{ resumeButtonLabel(session.status) }}
                </n-button>
              </n-space>
            </div>
          </div>

          <div v-if="showFlowSection" class="workflow-tabs-shell">
            <n-tabs v-model:value="activeTab" type="line" size="small" animated class="workflow-tabs">
              <n-tab-pane name="overview">
                <template #tab>概览</template>
                <div class="tab-pane">
                  <div class="tab-scroll">
                    <div class="overview-grid">
                      <div class="overview-stat">
                        <div class="overview-stat__label">执行链路</div>
                        <div class="overview-stat__value">{{ executions.length }}</div>
                      </div>
                      <div class="overview-stat">
                        <div class="overview-stat__label">运行中执行</div>
                        <div class="overview-stat__value">{{ runningExecutionCount }}</div>
                      </div>
                      <div class="overview-stat">
                        <div class="overview-stat__label">失败执行</div>
                        <div class="overview-stat__value">{{ failedExecutionCount }}</div>
                      </div>
                      <div class="overview-stat">
                        <div class="overview-stat__label">步骤总数</div>
                        <div class="overview-stat__value">{{ stepCount }}</div>
                      </div>
                    </div>

                    <div class="list-block">
                      <n-text depth="3" style="font-size: 12px">最新流程事件</n-text>
                      <n-empty v-if="!latestWorkflowEvent" size="small" description="暂无流程事件" />
                      <div v-else class="workflow-event-item">
                        <n-space align="center" size="small">
                          <n-tag size="small" :bordered="false" type="default">{{ latestWorkflowEvent.phase || 'lifecycle' }}</n-tag>
                          <n-tag size="small" :bordered="false" :type="workflowEventTagType(latestWorkflowEvent.event_type)">
                            {{ latestWorkflowEvent.event_type }}
                          </n-tag>
                          <n-text depth="3" style="font-size: 12px">{{ formatDateTime(latestWorkflowEvent.created_at) }}</n-text>
                        </n-space>
                        <n-text
                          v-if="safeText(latestWorkflowEvent.summary)"
                          depth="3"
                          style="display: block; font-size: 12px; margin-top: 6px"
                        >
                          {{ latestWorkflowEvent.summary }}
                        </n-text>
                      </div>
                    </div>

                    <div class="list-block">
                      <n-text depth="3" style="font-size: 12px">最新 CLI 日志</n-text>
                      <n-empty v-if="!latestWorkflowLog" size="small" description="暂无日志" />
                      <div v-else class="workflow-log-item">
                        <n-space align="center" size="small">
                          <n-tag size="small" :bordered="false" :type="workflowLogTagType(latestWorkflowLog.log_type)">
                            {{ workflowLogTypeLabel(latestWorkflowLog.log_type) }}
                          </n-tag>
                          <n-text depth="3" style="font-size: 12px">{{ formatDateTime(latestWorkflowLog.created_at) }}</n-text>
                        </n-space>
                        <pre class="payload-pre">{{ collapsedText(workflowLogContentKey(latestWorkflowLog), rawText(latestWorkflowLog.content), 900) || '—' }}</pre>
                        <n-button
                          v-if="shouldCollapseText(rawText(latestWorkflowLog.content), 900)"
                          class="payload-toggle"
                          size="tiny"
                          text
                          @click="toggleExpanded(workflowLogContentKey(latestWorkflowLog))"
                        >
                          {{ isExpanded(workflowLogContentKey(latestWorkflowLog)) ? '收起' : '展开' }}
                        </n-button>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tab-pane>

              <n-tab-pane name="executions">
                <template #tab>执行链路 ({{ executionDisplayItems.length }}/{{ executions.length }})</template>
                <div class="tab-pane">
                  <div class="tab-split">
                    <div class="list-block list-block--flex">
                      <div class="tab-toolbar">
                        <n-text depth="3" style="font-size: 12px">执行链路</n-text>
                        <n-space align="center" size="small" class="toolbar-controls">
                          <n-select
                            v-model:value="executionStatusFilter"
                            size="tiny"
                            class="toolbar-select toolbar-select--status"
                            :options="executionStatusOptions"
                          />
                          <n-select
                            v-model:value="executionRoleFilter"
                            size="tiny"
                            class="toolbar-select toolbar-select--type"
                            :options="executionRoleOptions"
                          />
                          <n-select
                            v-model:value="executionListLimit"
                            size="tiny"
                            class="toolbar-select toolbar-select--limit"
                            :options="executionListLimitOptions"
                          />
                          <n-button size="tiny" quaternary :disabled="executionsLoading" @click="loadExecutions(session.id, false)">
                            刷新
                          </n-button>
                          <n-spin v-if="executionsLoading" size="small" />
                        </n-space>
                      </div>
                      <n-empty v-if="!executionDisplayItems.length" size="small" description="暂无执行记录" />
                      <div v-else class="execution-list list-scroll">
                        <div
                          v-for="row in executionDisplayItems"
                          :key="row.item.id"
                          class="execution-item"
                          :class="{ 'execution-item--active': isExecutionSelected(row.item.id) }"
                          :style="{ paddingLeft: `${row.level * 14 + 8}px` }"
                          @click="selectExecution(row.item.id)"
                        >
                          <n-space align="center" size="small">
                            <n-tag size="small" :bordered="false" type="default">{{ executionRoleLabel(row.item.role) }}</n-tag>
                            <n-tag size="small" :bordered="false" :type="executionStatusTagType(row.item.status)">
                              {{ row.item.status }}
                            </n-tag>
                            <n-text depth="3" style="font-size: 12px">{{ row.item.mode || 'command' }}</n-text>
                            <n-text depth="3" style="font-size: 12px">{{ row.item.tool || 'shell' }}</n-text>
                          </n-space>
                          <n-text depth="3" style="display: block; font-size: 12px" class="mono">
                            {{ shortExecutionID(row.item.id) }}
                            <span v-if="row.item.parent_execution_id"> ← {{ shortExecutionID(row.item.parent_execution_id) }}</span>
                          </n-text>
                          <n-text
                            v-if="safeText(row.item.prompt_preview)"
                            depth="3"
                            class="execution-summary"
                          >
                            {{ row.item.prompt_preview }}
                          </n-text>
                        </div>
                      </div>
                    </div>

                    <div class="list-block list-block--flex">
                      <div class="tab-toolbar">
                        <n-text depth="3" style="font-size: 12px">
                          事件日志
                          <span v-if="selectedExecutionID" class="mono"> · {{ shortExecutionID(selectedExecutionID) }}</span>
                        </n-text>
                        <n-space align="center" size="small" class="toolbar-controls">
                          <n-select
                            v-model:value="executionEventLimit"
                            size="tiny"
                            class="toolbar-select toolbar-select--limit"
                            :options="executionEventLimitOptions"
                          />
                          <n-select
                            v-model:value="executionEventOrder"
                            size="tiny"
                            class="toolbar-select toolbar-select--order"
                            :options="orderOptions"
                          />
                          <n-button
                            size="tiny"
                            type="primary"
                            :loading="resumingExecution"
                            :disabled="!canResumeSelectedExecution"
                            @click="resumeSelectedExecution"
                          >
                            CLI会话恢复
                          </n-button>
                          <n-button
                            size="tiny"
                            quaternary
                            :disabled="!selectedExecutionID || executionEventsLoading"
                            @click="loadExecutionEvents(selectedExecutionID, false)"
                          >
                            刷新事件
                          </n-button>
                          <n-spin v-if="executionEventsLoading" size="small" />
                        </n-space>
                      </div>
                      <n-empty v-if="!selectedExecutionID" size="small" description="请选择执行链路" />
                      <n-empty v-else-if="!executionEventDisplayItems.length" size="small" description="暂无事件" />
                      <div v-else class="execution-event-list list-scroll">
                        <div v-for="event in executionEventDisplayItems" :key="event.seq" class="execution-event-item">
                          <n-space align="center" size="small">
                            <n-tag size="small" :bordered="false" type="info">{{ event.event_type }}</n-tag>
                            <n-text depth="3" style="font-size: 12px">#{{ event.seq }}</n-text>
                            <n-text depth="3" style="font-size: 12px">{{ formatDateTime(event.created_at) }}</n-text>
                          </n-space>
                          <template v-if="executionEventPayload(event)">
                            <pre class="payload-pre">{{ collapsedText(executionEventPayloadKey(event), executionEventPayload(event), 1200) }}</pre>
                            <n-button
                              v-if="shouldCollapseText(executionEventPayload(event), 1200)"
                              class="payload-toggle"
                              size="tiny"
                              text
                              @click="toggleExpanded(executionEventPayloadKey(event))"
                            >
                              {{ isExpanded(executionEventPayloadKey(event)) ? '收起' : '展开' }}
                            </n-button>
                          </template>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tab-pane>

              <n-tab-pane name="events">
                <template #tab>流程事件 ({{ workflowEvents.length }})</template>
                <div class="tab-pane">
                  <div class="tab-layout">
                    <div class="tab-toolbar">
                      <n-text depth="3" style="font-size: 12px">流程事件</n-text>
                      <n-space align="center" size="small" class="toolbar-controls">
                        <n-select
                          v-model:value="workflowEventsOrder"
                          size="tiny"
                          class="toolbar-select toolbar-select--order"
                          :options="orderOptions"
                        />
                        <n-select
                          v-model:value="workflowEventsLimit"
                          size="tiny"
                          class="toolbar-select toolbar-select--limit"
                          :options="workflowEventsLimitOptions"
                        />
                        <n-button size="tiny" quaternary @click="jumpToLatestWorkflowEvents">跳到最新</n-button>
                        <n-button
                          size="tiny"
                          quaternary
                          :disabled="workflowEventsLoading"
                          @click="loadWorkflowEvents(session.id, false)"
                        >
                          刷新事件
                        </n-button>
                        <n-spin v-if="workflowEventsLoading" size="small" />
                      </n-space>
                    </div>
                    <div ref="workflowEventsContainer" class="tab-scroll">
                      <n-empty v-if="!workflowEventDisplayItems.length" size="small" description="暂无流程事件" />
                      <div v-else class="workflow-event-list">
                        <div v-for="event in workflowEventDisplayItems" :key="event.id" class="workflow-event-item">
                          <n-space align="center" size="small">
                            <n-tag size="small" :bordered="false" type="default">{{ event.phase || 'lifecycle' }}</n-tag>
                            <n-tag size="small" :bordered="false" :type="workflowEventTagType(event.event_type)">
                              {{ event.event_type }}
                            </n-tag>
                            <n-text depth="3" style="font-size: 12px">
                              #{{ Number.isFinite(event.iteration) && event.iteration >= 0 ? event.iteration + 1 : '—' }}
                            </n-text>
                            <n-text depth="3" style="font-size: 12px">{{ formatDateTime(event.created_at) }}</n-text>
                          </n-space>
                          <n-text
                            v-if="safeText(event.summary)"
                            depth="3"
                            style="display: block; font-size: 12px; margin-top: 4px"
                          >
                            {{ event.summary }}
                          </n-text>
                          <template v-if="workflowEventPayload(event)">
                            <pre class="payload-pre">{{ collapsedText(workflowEventPayloadKey(event), workflowEventPayload(event), 1400) }}</pre>
                            <n-button
                              v-if="shouldCollapseText(workflowEventPayload(event), 1400)"
                              class="payload-toggle"
                              size="tiny"
                              text
                              @click="toggleExpanded(workflowEventPayloadKey(event))"
                            >
                              {{ isExpanded(workflowEventPayloadKey(event)) ? '收起' : '展开' }}
                            </n-button>
                          </template>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tab-pane>

              <n-tab-pane name="logs">
                <template #tab>CLI日志 ({{ workflowLogs.length }})</template>
                <div class="tab-pane">
                  <div class="tab-layout">
                    <div class="tab-toolbar">
                      <n-text depth="3" style="font-size: 12px">CLI I/O 日志</n-text>
                      <n-space align="center" size="small" class="toolbar-controls">
                        <n-select
                          v-model:value="workflowLogsType"
                          :options="workflowLogTypeOptions"
                          size="tiny"
                          class="toolbar-select toolbar-select--type"
                        />
                        <n-select
                          v-model:value="workflowLogsOrder"
                          size="tiny"
                          class="toolbar-select toolbar-select--order"
                          :options="orderOptions"
                        />
                        <n-select
                          v-model:value="workflowLogsLimit"
                          size="tiny"
                          class="toolbar-select toolbar-select--limit"
                          :options="workflowLogsLimitOptions"
                        />
                        <n-button size="tiny" quaternary @click="jumpToLatestWorkflowLogs">跳到最新</n-button>
                        <n-button
                          size="tiny"
                          quaternary
                          :disabled="workflowLogsLoading"
                          @click="loadWorkflowLogs(session.id, false)"
                        >
                          刷新日志
                        </n-button>
                        <n-spin v-if="workflowLogsLoading" size="small" />
                      </n-space>
                    </div>
                    <div ref="workflowLogsContainer" class="tab-scroll">
                      <n-empty v-if="!workflowLogs.length" size="small" description="暂无日志" />
                      <div v-else class="workflow-log-list">
                        <div v-for="log in workflowLogs" :key="log.id" class="workflow-log-item">
                          <n-space align="center" size="small">
                            <n-tag size="small" :bordered="false" :type="workflowLogTagType(log.log_type)">
                              {{ workflowLogTypeLabel(log.log_type) }}
                            </n-tag>
                            <n-text depth="3" style="font-size: 12px">{{ formatDateTime(log.created_at) }}</n-text>
                          </n-space>
                          <pre class="payload-pre">{{ collapsedText(workflowLogContentKey(log), rawText(log.content), 1800) || '—' }}</pre>
                          <n-button
                            v-if="shouldCollapseText(rawText(log.content), 1800)"
                            class="payload-toggle"
                            size="tiny"
                            text
                            @click="toggleExpanded(workflowLogContentKey(log))"
                          >
                            {{ isExpanded(workflowLogContentKey(log)) ? '收起' : '展开' }}
                          </n-button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tab-pane>

              <n-tab-pane name="steps">
                <template #tab>步骤 ({{ stepsDisplayItems.length }}/{{ stepCount }})</template>
                <div class="tab-pane tab-pane--steps">
                  <div class="tab-layout">
                    <div class="tab-toolbar">
                      <n-text depth="3" style="font-size: 12px">步骤明细</n-text>
                      <n-space align="center" size="small" class="toolbar-controls">
                        <n-select
                          v-model:value="stepsStatusFilter"
                          size="tiny"
                          class="toolbar-select toolbar-select--status"
                          :options="stepStatusOptions"
                        />
                        <n-select
                          v-model:value="stepsLimit"
                          size="tiny"
                          class="toolbar-select toolbar-select--limit"
                          :options="stepsLimitOptions"
                        />
                      </n-space>
                    </div>
                    <div ref="stepsContainer" class="steps-container tab-scroll" @scroll="handleScroll">
                      <n-empty v-if="!stepsDisplayItems.length" description="暂无步骤" />
                      <div v-else class="steps">
                        <div v-for="step in stepsDisplayItems" :key="step.id" class="step">
                          <div class="step__header">
                            <n-space align="center" size="small">
                              <n-tag size="small" :bordered="false" type="default">
                                #{{ Number.isFinite(step.iteration) ? step.iteration + 1 : '—' }}
                              </n-tag>
                              <n-tag size="small" :bordered="false" :type="step.success ? 'success' : 'error'">
                                {{ step.success ? 'success' : 'failed' }}
                              </n-tag>
                              <n-text depth="3" style="font-size: 12px">
                                {{ formatDateTime(step.timestamp) }}
                              </n-text>
                            </n-space>
                          </div>
                          <div class="step__row">
                            <div class="step__label">action</div>
                            <div class="step__content mono">{{ step.action || '—' }}</div>
                          </div>
                          <div class="step__row">
                            <div class="step__label">result</div>
                            <div>
                              <pre class="payload-pre">{{ collapsedText(stepResultKey(step), step.result || '—', 2200) }}</pre>
                              <n-button
                                v-if="shouldCollapseText(step.result || '', 2200)"
                                class="payload-toggle"
                                size="tiny"
                                text
                                @click="toggleExpanded(stepResultKey(step))"
                              >
                                {{ isExpanded(stepResultKey(step)) ? '收起' : '展开' }}
                              </n-button>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-if="showScrollToBottom && stepsDisplayItems.length" class="scroll-bottom">
                    <n-button size="tiny" type="primary" @click="scrollToBottom">回到底部</n-button>
                  </div>
                </div>
              </n-tab-pane>
            </n-tabs>
          </div>
        </template>
      </div>
    </n-spin>
  </n-card>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  getAIWorkflowSession,
  getAIWorkflowSessionEvents,
  getAIWorkflowSessionLogs,
  postAIWorkflowMessage,
  postAIWorkflowPause,
  type AIWorkflowEventRecord,
  type AIWorkflowLogRecord,
  type AIWorkflowSession,
  type AIWorkflowStep
} from '@/api/ai-workflow'
import { listCLIExecutionEvents, listCLIExecutions, resumeCLIExecution, streamCLIExecutionEvents } from '@/api/cli-execution'
import type { CLIExecution, CLIExecutionEvent, CLIExecutionSSEEnvelope } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const props = withDefaults(defineProps<{
  sessionId: string
  panelMode?: 'full' | 'control' | 'flow'
  externalScroll?: boolean
}>(), {
  panelMode: 'full',
  externalScroll: false
})

const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)

type SessionPanelTab = 'overview' | 'executions' | 'events' | 'logs' | 'steps'

const session = ref<AIWorkflowSession | null>(null)
const loading = ref(false)
const resuming = ref(false)
const pausing = ref(false)
const resumeMessage = ref('')
const activeTab = ref<SessionPanelTab>('overview')
const contextCollapsed = ref(false)
const stepsContainer = ref<HTMLElement | null>(null)
const workflowEventsContainer = ref<HTMLElement | null>(null)
const workflowLogsContainer = ref<HTMLElement | null>(null)
const showScrollToBottom = ref(false)
const executions = ref<CLIExecution[]>([])
const executionsInitialized = ref(false)
const executionsLoading = ref(false)
const workflowEvents = ref<AIWorkflowEventRecord[]>([])
const workflowEventsLoading = ref(false)
const workflowLogs = ref<AIWorkflowLogRecord[]>([])
const workflowLogsInitialized = ref(false)
const workflowLogsLoading = ref(false)
const workflowLogsType = ref(defaultWorkflowLogsType())
const workflowLogsLimit = ref(200)
const workflowLogsOrder = ref<'asc' | 'desc'>('desc')
const workflowEventsLimit = ref(200)
const workflowEventsOrder = ref<'asc' | 'desc'>('asc')
const workflowEventsLastID = ref(0)
const executionStatusFilter = ref('')
const executionRoleFilter = ref('')
const executionListLimit = ref(50)
const executionEventLimit = ref(200)
const executionEventOrder = ref<'asc' | 'desc'>('asc')
const stepsStatusFilter = ref('')
const stepsLimit = ref(200)
const selectedExecutionID = ref('')
const executionEvents = ref<CLIExecutionEvent[]>([])
const executionEventsLoading = ref(false)
const resumingExecution = ref(false)
const payloadExpandedMap = ref<Record<string, boolean>>({})
const executionStreams = new Map<string, AbortController>()
const executionStreamCursor = new Map<string, number>()
let executionReloadTimer: number | null = null
let executionEventReloadTimer: number | null = null
let executionEventsRequestSeq = 0
let workflowEventsRequestSeq = 0
let workflowLogsRequestSeq = 0
const pollIntervalMs = 2500
let pollTimer: number | null = null

const sessionTerminalID = computed(() => safeText(session.value?.context?.terminal_id))
const sessionGoalText = computed(() => safeText(session.value?.user_goal))
const sessionSummaryText = computed(() => safeText(session.value?.summary))
const showContextSection = computed(() => props.panelMode !== 'flow')
const showFlowSection = computed(() => props.panelMode !== 'control')
const canToggleContext = computed(() => showContextSection.value && showFlowSection.value)
const hasSessionContext = computed(() =>
  !!sessionGoalText.value || !!sessionSummaryText.value || canResume(session.value?.status || '')
)
const selectedExecution = computed(() =>
  executions.value.find(item => safeText(item.id) === safeText(selectedExecutionID.value)) || null
)
const canResumeSelectedExecution = computed(() => {
  if (isDemoMode.value || resumingExecution.value) {
    return false
  }
  const status = safeText(selectedExecution.value?.status || '').toLowerCase()
  return status !== '' && status !== 'running'
})
const canPauseSession = computed(() => {
  if (props.panelMode === 'flow') {
    return false
  }
  if (isDemoMode.value || pausing.value) {
    return false
  }
  const status = safeText(session.value?.status || '').toLowerCase()
  return status === 'running'
})
const orderOptions = [
  { label: '正序', value: 'asc' },
  { label: '倒序', value: 'desc' }
]
const workflowEventsLimitOptions = [
  { label: '50条', value: 50 },
  { label: '100条', value: 100 },
  { label: '200条', value: 200 },
  { label: '500条', value: 500 }
]
const workflowLogsLimitOptions = [
  { label: '100条', value: 100 },
  { label: '200条', value: 200 },
  { label: '500条', value: 500 },
  { label: '1000条', value: 1000 }
]
const executionListLimitOptions = [
  { label: '20条', value: 20 },
  { label: '50条', value: 50 },
  { label: '100条', value: 100 },
  { label: '200条', value: 200 }
]
const executionEventLimitOptions = [
  { label: '100条', value: 100 },
  { label: '200条', value: 200 },
  { label: '500条', value: 500 },
  { label: '1000条', value: 1000 }
]
const stepsLimitOptions = [
  { label: '50条', value: 50 },
  { label: '100条', value: 100 },
  { label: '200条', value: 200 },
  { label: '500条', value: 500 }
]
const executionStatusOptions = [
  { label: '状态: 全部', value: '' },
  { label: '运行中', value: 'running' },
  { label: '已完成', value: 'completed' },
  { label: '异常', value: 'error' },
  { label: '失败', value: 'failed' },
  { label: '超时', value: 'timeout' },
  { label: '已取消', value: 'cancelled' }
]
const executionRoleOptions = [
  { label: '角色: 全部', value: '' },
  { label: 'primary', value: 'primary' },
  { label: 'review', value: 'review' },
  { label: 'replay', value: 'replay' },
  { label: 'audit', value: 'audit' }
]
const stepStatusOptions = [
  { label: '状态: 全部', value: '' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' }
]
const workflowLogTypeOptions = [
  { label: '全部', value: '' },
  { label: 'AI原生(对话)', value: 'ai_native_all' },
  { label: 'AI原生输入', value: 'ai_input_native' },
  { label: 'AI原生输出', value: 'ai_output_native' },
  { label: '输入(raw)', value: 'input_raw' },
  { label: '输出(raw)', value: 'output_raw' },
  { label: '输入(聚合)', value: 'input' },
  { label: '输出(聚合)', value: 'output' },
  { label: '系统', value: 'system' }
]

function defaultWorkflowLogsType() {
  return props.panelMode === 'flow' ? 'ai_native_all' : ''
}

type ExecutionTreeRow = { item: CLIExecution; level: number }
const executionDisplayItems = computed<ExecutionTreeRow[]>(() => {
  if (!executions.value.length) {
    return []
  }

  const byID = new Map<string, CLIExecution>()
  const children = new Map<string, CLIExecution[]>()
  const roots: CLIExecution[] = []

  const sortByStartedAt = (a: CLIExecution, b: CLIExecution) => {
    const ta = Date.parse(safeText(a.started_at))
    const tb = Date.parse(safeText(b.started_at))
    if (Number.isNaN(ta) || Number.isNaN(tb)) {
      return safeText(a.id).localeCompare(safeText(b.id))
    }
    return tb - ta
  }

  for (const item of executions.value) {
    byID.set(item.id, item)
  }
  for (const item of executions.value) {
    const parentID = safeText(item.parent_execution_id || '')
    if (!parentID || !byID.has(parentID)) {
      roots.push(item)
      continue
    }
    const group = children.get(parentID) || []
    group.push(item)
    children.set(parentID, group)
  }

  roots.sort(sortByStartedAt)
  for (const [, group] of children) {
    group.sort(sortByStartedAt)
  }

  const rows: ExecutionTreeRow[] = []
  const visited = new Set<string>()
  const walk = (node: CLIExecution, level: number) => {
    if (!node || visited.has(node.id)) {
      return
    }
    visited.add(node.id)
    rows.push({ item: node, level })
    const group = children.get(node.id) || []
    for (const child of group) {
      walk(child, level + 1)
    }
  }

  for (const root of roots) {
    walk(root, 0)
  }

  let filteredRows = rows
  const statusFilter = safeText(executionStatusFilter.value).toLowerCase()
  const roleFilter = safeText(executionRoleFilter.value).toLowerCase()

  if (statusFilter) {
    filteredRows = filteredRows.filter((row) => safeText(row.item.status).toLowerCase() === statusFilter)
  }
  if (roleFilter) {
    filteredRows = filteredRows.filter((row) => safeText(row.item.role).toLowerCase() === roleFilter)
  }

  const limit = Number(executionListLimit.value)
  if (Number.isFinite(limit) && limit > 0 && filteredRows.length > limit) {
    filteredRows = filteredRows.slice(0, limit)
  }

  return filteredRows
})

const stepCount = computed(() => session.value?.steps?.length || 0)
const stepsDisplayItems = computed(() => {
  const sourceSteps = Array.isArray(session.value?.steps) ? session.value?.steps || [] : []
  let items = [...sourceSteps]
  const statusFilter = safeText(stepsStatusFilter.value).toLowerCase()
  if (statusFilter === 'success') {
    items = items.filter((step) => step.success)
  } else if (statusFilter === 'failed') {
    items = items.filter((step) => !step.success)
  }

  const limit = Number(stepsLimit.value)
  if (Number.isFinite(limit) && limit > 0 && items.length > limit) {
    items = items.slice(items.length - limit)
  }
  return items
})
const runningExecutionCount = computed(() =>
  executions.value.filter(item => safeText(item.status).toLowerCase() === 'running').length
)
const failedExecutionCount = computed(() =>
  executions.value.filter((item) => {
    const status = safeText(item.status).toLowerCase()
    return status === 'failed' || status === 'error' || status === 'cancelled' || status === 'timeout'
  }).length
)
const workflowEventDisplayItems = computed(() => {
  const items = workflowEvents.value
  if (workflowEventsOrder.value === 'desc') {
    return [...items].reverse()
  }
  return items
})
const executionEventDisplayItems = computed(() => {
  if (executionEventOrder.value === 'desc') {
    return [...executionEvents.value].reverse()
  }
  return executionEvents.value
})
const latestWorkflowEvent = computed(() => pickLatestByCreatedAt(workflowEvents.value))
const latestWorkflowLog = computed(() => pickLatestByCreatedAt(workflowLogs.value))

const resumableStatuses = new Set(['paused', 'completed', 'failed', 'cancelled'])
let pollInFlight = false
let requestSeq = 0

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs = 15000): Promise<T> {
  let timer: number | null = null
  const timeoutPromise = new Promise<T>((_, reject) => {
    timer = window.setTimeout(() => reject(new Error('request timeout')), timeoutMs)
  })
  try {
    return await Promise.race([promise, timeoutPromise]) as T
  } finally {
    if (timer) {
      window.clearTimeout(timer)
    }
  }
}

function rawText(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function pickLatestByCreatedAt<T extends { created_at?: unknown }>(items: T[]) {
  let latest: T | null = null
  let latestTs = 0

  for (const item of items) {
    const ts = Date.parse(safeText(item.created_at))
    if (Number.isNaN(ts)) {
      if (!latest) {
        latest = item
      }
      continue
    }
    if (!latest || ts > latestTs) {
      latest = item
      latestTs = ts
    }
  }

  return latest
}

function formatDateTime(value: unknown) {
  const raw = safeText(value)
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString('zh-CN')
}

function statusTagType(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'completed') return 'success'
  if (s === 'running') return 'info'
  if (s === 'paused') return 'warning'
  if (s === 'failed' || s === 'error') return 'error'
  return 'default'
}

function executionStatusTagType(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'completed') return 'success'
  if (s === 'running') return 'info'
  if (s === 'error' || s === 'failed' || s === 'timeout' || s === 'cancelled') return 'error'
  return 'default'
}

function executionRoleLabel(role: string) {
  const r = safeText(role).toLowerCase()
  if (r === 'review') return 'review'
  if (r === 'replay') return 'replay'
  if (r === 'audit') return 'audit'
  return 'primary'
}

function shortExecutionID(id: string | null | undefined) {
  const text = safeText(id || '')
  if (!text) return '—'
  if (text.length <= 16) return text
  return `${text.slice(0, 8)}...${text.slice(-6)}`
}

function formatExecutionEventPayload(payload: unknown) {
  if (payload == null) {
    return ''
  }
  if (typeof payload === 'string') {
    return payload
  }
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
}

function formatWorkflowEventPayload(payload: unknown) {
  const text = formatExecutionEventPayload(payload)
  if (!text) {
    return ''
  }
  const maxLength = 8000
  if (text.length <= maxLength) {
    return text
  }
  return `${text.slice(0, maxLength)}\n...(truncated)`
}

function workflowEventTagType(eventType: string) {
  const t = safeText(eventType).toLowerCase()
  if (t.includes('error') || t.includes('failed')) return 'error'
  if (t.includes('paused') || t.includes('need_user')) return 'warning'
  if (t.includes('completed') || t.includes('result')) return 'success'
  if (t.includes('tool_call') || t.includes('step_started')) return 'info'
  return 'default'
}

function workflowLogTypeLabel(logType: string) {
  const t = safeText(logType).toLowerCase()
  if (t === 'ai_input_native') return 'ai_input_native'
  if (t === 'ai_output_native') return 'ai_output_native'
  if (t === 'input_raw') return 'input_raw'
  if (t === 'output_raw') return 'output_raw'
  if (t === 'input') return 'input'
  if (t === 'output') return 'output'
  if (t === 'system') return 'system'
  return t || 'log'
}

function workflowLogTagType(logType: string) {
  const t = safeText(logType).toLowerCase()
  if (t.includes('input')) return 'info'
  if (t.includes('output')) return 'success'
  if (t === 'system') return 'default'
  return 'default'
}

function shouldCollapseText(text: string, limit = 1200) {
  return text.length > limit
}

function isExpanded(key: string) {
  return !!payloadExpandedMap.value[key]
}

function toggleExpanded(key: string) {
  payloadExpandedMap.value[key] = !payloadExpandedMap.value[key]
}

function collapsedText(key: string, rawText: string, limit = 1200) {
  if (!rawText) {
    return ''
  }
  if (!shouldCollapseText(rawText, limit) || isExpanded(key)) {
    return rawText
  }
  return `${rawText.slice(0, limit)}\n...(已折叠，点击展开)`
}

function workflowLogContentKey(log: AIWorkflowLogRecord) {
  return `workflow-log:${safeText(log.id)}`
}

function workflowEventPayloadKey(event: AIWorkflowEventRecord) {
  return `workflow-event:${event.id}`
}

function executionEventPayloadKey(event: CLIExecutionEvent) {
  return `execution-event:${safeText(selectedExecutionID.value)}:${event.seq}`
}

function stepResultKey(step: AIWorkflowStep) {
  return `step-result:${safeText(step.id)}`
}

function executionEventPayload(event: CLIExecutionEvent) {
  return formatExecutionEventPayload(event.payload)
}

function workflowEventPayload(event: AIWorkflowEventRecord) {
  return formatWorkflowEventPayload(event.payload)
}

function clearExecutionEventReloadTimer() {
  if (executionEventReloadTimer) {
    window.clearTimeout(executionEventReloadTimer)
    executionEventReloadTimer = null
  }
}

function scheduleExecutionEventsReload(id?: string) {
  const executionID = safeText(id || selectedExecutionID.value)
  if (!executionID) {
    return
  }
  clearExecutionEventReloadTimer()
  executionEventReloadTimer = window.setTimeout(async () => {
    executionEventReloadTimer = null
    if (safeText(selectedExecutionID.value) !== executionID) {
      return
    }
    await loadExecutionEvents(executionID, true)
  }, 350)
}

function scheduleExecutionReload() {
  if (executionReloadTimer) {
    window.clearTimeout(executionReloadTimer)
  }
  executionReloadTimer = window.setTimeout(async () => {
    executionReloadTimer = null
    await refresh(true)
  }, 400)
}

function stopExecutionStreams() {
  for (const [, controller] of executionStreams) {
    controller.abort()
  }
  executionStreams.clear()
  executionStreamCursor.clear()
  clearExecutionEventReloadTimer()
  if (executionReloadTimer) {
    window.clearTimeout(executionReloadTimer)
    executionReloadTimer = null
  }
}

function extractStreamCursor(event: CLIExecutionSSEEnvelope): number {
  const payload = event?.data as any
  if (event.event === 'message') {
    const seq = Number(payload?.seq)
    return Number.isFinite(seq) && seq > 0 ? Math.floor(seq) : 0
  }
  if (event.event === 'ready') {
    const after = Number(payload?.after)
    return Number.isFinite(after) && after >= 0 ? Math.floor(after) : 0
  }
  return 0
}

function startExecutionStream(executionID: string) {
  const id = safeText(executionID)
  if (!id || executionStreams.has(id)) {
    return
  }

  const controller = new AbortController()
  executionStreams.set(id, controller)
  const afterCursor = executionStreamCursor.get(id) ?? 0

  streamCLIExecutionEvents(id, {
    signal: controller.signal,
    params: { after: afterCursor, limit: 200, poll_ms: 800, timeout_sec: 90 },
    onEvent: (evt) => {
      const nextCursor = extractStreamCursor(evt)
      if (nextCursor > 0) {
        const current = executionStreamCursor.get(id) ?? 0
        if (nextCursor > current) {
          executionStreamCursor.set(id, nextCursor)
        }
      }
      if (evt.event === 'message' || evt.event === 'done' || evt.event === 'timeout' || evt.event === 'error') {
        scheduleExecutionReload()
        if (safeText(selectedExecutionID.value) === id) {
          scheduleExecutionEventsReload(id)
        }
      }
    }
  })
    .catch(() => {
      // Ignore transient network/abort errors; polling remains fallback.
    })
    .finally(() => {
      if (executionStreams.get(id) === controller) {
        executionStreams.delete(id)
      }
    })
}

function syncExecutionStreams() {
  const trackedIDs = new Set(executions.value.map((item) => safeText(item.id)).filter(Boolean))
  for (const [id] of executionStreamCursor) {
    if (!trackedIDs.has(id)) {
      executionStreamCursor.delete(id)
    }
  }

  const runningIDs = new Set(
    executions.value
      .filter((item) => safeText(item.status).toLowerCase() === 'running')
      .map((item) => safeText(item.id))
      .filter(Boolean)
  )

  for (const [id, controller] of executionStreams) {
    if (!runningIDs.has(id)) {
      controller.abort()
      executionStreams.delete(id)
    }
  }

  for (const id of runningIDs) {
    startExecutionStream(id)
  }
}

function isExecutionSelected(executionID: string) {
  return safeText(executionID) !== '' && safeText(selectedExecutionID.value) === safeText(executionID)
}

function selectExecution(executionID: string) {
  const id = safeText(executionID)
  if (!id || id === safeText(selectedExecutionID.value)) {
    return
  }
  selectedExecutionID.value = id
  executionEvents.value = []
  void loadExecutionEvents(id, true)
}

function syncSelectedExecution(items: CLIExecution[]) {
  const validIDs = new Set(items.map((item) => safeText(item.id)).filter(Boolean))
  const currentID = safeText(selectedExecutionID.value)

  if (!validIDs.size) {
    selectedExecutionID.value = ''
    executionEvents.value = []
    executionEventsLoading.value = false
    executionEventsRequestSeq++
    clearExecutionEventReloadTimer()
    return
  }

  if (!currentID || !validIDs.has(currentID)) {
    const nextID = safeText(items[0]?.id || '')
    selectedExecutionID.value = nextID
    executionEvents.value = []
    if (nextID) {
      void loadExecutionEvents(nextID, true)
    }
  }
}

async function loadExecutionEvents(executionID: string, silent = true) {
  const id = safeText(executionID)
  if (!id) {
    executionEvents.value = []
    return
  }

  const seq = ++executionEventsRequestSeq
  if (!silent) {
    executionEventsLoading.value = true
  }
  try {
    const limit = Number(executionEventLimit.value) || 200
    const { data } = await listCLIExecutionEvents(id, { limit })
    if (seq !== executionEventsRequestSeq || safeText(selectedExecutionID.value) !== id) {
      return
    }
    const items = Array.isArray(data?.items) ? (data.items as CLIExecutionEvent[]) : []
    executionEvents.value = items
  } catch (e: any) {
    if (!silent) {
      message.error(e?.response?.data?.error || '获取执行事件失败')
    }
  } finally {
    if (!silent) {
      executionEventsLoading.value = false
    }
  }
}

async function loadWorkflowEvents(workflowSessionID: string, silent = true) {
  const sid = safeText(workflowSessionID)
  if (!sid) {
    workflowEvents.value = []
    workflowEventsLoading.value = false
    workflowEventsLastID.value = 0
    workflowEventsRequestSeq++
    return
  }

  const seq = ++workflowEventsRequestSeq
  if (!silent) {
    workflowEventsLoading.value = true
  }
  try {
    const limit = Number(workflowEventsLimit.value) || 200
    const shouldFullReload = !silent || workflowEvents.value.length === 0
    let cursor = shouldFullReload ? 0 : Number(workflowEventsLastID.value || 0)
    const pages: AIWorkflowEventRecord[] = []
    const maxPages = 20

    for (let i = 0; i < maxPages; i++) {
      const params: { limit: number; after_id?: number } = { limit }
      if (cursor > 0) {
        params.after_id = cursor
      }
      const { data } = await getAIWorkflowSessionEvents(sid, params)
      if (seq !== workflowEventsRequestSeq || safeText(session.value?.id) !== sid) {
        return
      }
      const chunk = Array.isArray(data?.items) ? (data.items as AIWorkflowEventRecord[]) : []
      if (!chunk.length) {
        break
      }
      pages.push(...chunk)

      const responseLastID = Number(data?.last_id)
      const chunkLastID = Number(chunk[chunk.length - 1]?.id || 0)
      const nextCursor = Number.isFinite(responseLastID) && responseLastID > cursor
        ? Math.floor(responseLastID)
        : Math.max(cursor, chunkLastID)
      if (nextCursor <= cursor) {
        break
      }
      cursor = nextCursor
      if (!data?.has_more) {
        break
      }
    }

    if (shouldFullReload) {
      workflowEvents.value = pages
    } else if (pages.length) {
      const seen = new Set(workflowEvents.value.map((item) => Number(item.id)))
      const appended = pages.filter((item) => {
        const id = Number(item.id)
        if (!Number.isFinite(id) || seen.has(id)) {
          return false
        }
        seen.add(id)
        return true
      })
      if (appended.length) {
        workflowEvents.value = [...workflowEvents.value, ...appended]
      }
    }

    if (pages.length) {
      const nextLastID = Math.max(...pages.map((item) => Number(item.id) || 0))
      workflowEventsLastID.value = shouldFullReload
        ? nextLastID
        : Math.max(Number(workflowEventsLastID.value || 0), nextLastID)
    } else if (shouldFullReload) {
      workflowEventsLastID.value = 0
    }
  } catch (e: any) {
    if (!silent) {
      message.error(e?.response?.data?.error || '获取流程事件失败')
    }
  } finally {
    if (!silent) {
      workflowEventsLoading.value = false
    }
  }
}

async function loadWorkflowLogs(workflowSessionID: string, silent = true) {
  const sid = safeText(workflowSessionID)
  if (!sid) {
    workflowLogs.value = []
    workflowLogsInitialized.value = false
    workflowLogsLoading.value = false
    workflowLogsRequestSeq++
    return
  }

  const seq = ++workflowLogsRequestSeq
  if (!silent) {
    workflowLogsLoading.value = true
  }
  try {
    const selectedType = safeText(workflowLogsType.value)
    const includeRaw = selectedType === 'input_raw' || selectedType === 'output_raw'
    const source = selectedType === 'ai_native_all' ? 'native' : undefined
    const type = selectedType === 'ai_native_all' ? undefined : selectedType || undefined
    const limit = Number(workflowLogsLimit.value) || 200
    const { data } = await getAIWorkflowSessionLogs(sid, {
      limit,
      offset: 0,
      order: workflowLogsOrder.value,
      source,
      type,
      include_raw: includeRaw
    })
    if (seq !== workflowLogsRequestSeq || safeText(session.value?.id) !== sid) {
      return
    }
    workflowLogs.value = Array.isArray(data?.items) ? (data.items as AIWorkflowLogRecord[]) : []
    workflowLogsInitialized.value = true
  } catch (e: any) {
    if (!silent) {
      message.error(e?.response?.data?.error || '获取 CLI 日志失败')
    }
  } finally {
    if (!silent) {
      workflowLogsLoading.value = false
    }
  }
}

function canResume(status: string) {
  return resumableStatuses.has(safeText(status).toLowerCase())
}

function toggleContextCollapsed() {
  contextCollapsed.value = !contextCollapsed.value
}

function resumeButtonLabel(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'paused') return '继续执行'
  return '继续对话'
}

function resumePlaceholder(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'paused') return '补充信息/确认后继续（Ctrl+Enter 发送）'
  return '追加要求/复查说明（Ctrl+Enter 发送）'
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPolling() {
  stopPolling()
  pollTimer = window.setInterval(() => {
    const sid = safeText(session.value?.id || props.sessionId)
    if (!sid) {
      return
    }
    const status = safeText(session.value?.status || '').toLowerCase()
    if (status !== '' && status !== 'running') {
      return
    }
    void refresh(true)
  }, pollIntervalMs)
}

function isNearBottom(threshold = 80) {
  const el = stepsContainer.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight < threshold
}

function nextAnimationFrame() {
  return new Promise<void>(resolve => {
    requestAnimationFrame(() => resolve())
  })
}

async function scrollToBottom() {
  const el = stepsContainer.value
  if (!el) return
  await nextTick()
  await nextAnimationFrame()
  await nextAnimationFrame()
  const maxScrollTop = Math.max(0, el.scrollHeight - el.clientHeight)
  el.scrollTop = maxScrollTop
  showScrollToBottom.value = false
}

function handleScroll() {
  showScrollToBottom.value = !isNearBottom()
}

function jumpToLatestWorkflowEvents() {
  const el = workflowEventsContainer.value
  if (!el) {
    return
  }
  el.scrollTop = workflowEventsOrder.value === 'desc' ? 0 : el.scrollHeight
}

function jumpToLatestWorkflowLogs() {
  const el = workflowLogsContainer.value
  if (!el) {
    return
  }
  el.scrollTop = workflowLogsOrder.value === 'desc' ? 0 : el.scrollHeight
}

async function refresh(silent = true) {
  const id = safeText(props.sessionId)
  if (!id || pollInFlight) return
  pollInFlight = true
  const seq = ++requestSeq
  if (!silent) {
    loading.value = true
  }
  try {
    const shouldStick = activeTab.value === 'steps' && (!showScrollToBottom.value || isNearBottom())
    const { data } = await getAIWorkflowSession(id)
    if (seq !== requestSeq) return
    session.value = (data?.session as AIWorkflowSession) || null
    if (session.value?.id) {
      const shouldLoadLogs = !silent || !workflowLogsInitialized.value || workflowLogs.value.length > 0
      const shouldLoadExecutions = !silent || !executionsInitialized.value || executions.value.length > 0
      const jobs: Array<Promise<void>> = [
        loadWorkflowEvents(session.value.id, true)
      ]
      if (shouldLoadLogs) {
        jobs.push(loadWorkflowLogs(session.value.id, true))
      }
      if (shouldLoadExecutions) {
        jobs.unshift(loadExecutions(session.value.id, true))
      }
      await Promise.all(jobs)
    } else {
      executions.value = []
      executionsInitialized.value = false
      workflowEvents.value = []
      workflowLogs.value = []
      workflowLogsInitialized.value = false
      workflowEventsRequestSeq++
      workflowLogsRequestSeq++
    }

    if (activeTab.value === 'steps') {
      if (shouldStick) {
        await scrollToBottom()
      } else {
        await nextTick()
        showScrollToBottom.value = !isNearBottom()
      }
    }
  } catch (e: any) {
    if (!silent) message.error(e?.response?.data?.error || '获取会话失败')
  } finally {
    if (!silent) loading.value = false
    pollInFlight = false
  }
}

async function loadExecutions(workflowSessionID: string, silent = true) {
  const sid = safeText(workflowSessionID)
  if (!sid) {
    stopExecutionStreams()
    executions.value = []
    executionsInitialized.value = false
    selectedExecutionID.value = ''
    executionEvents.value = []
    executionEventsLoading.value = false
    executionEventsRequestSeq++
    return
  }

  if (!silent) {
    executionsLoading.value = true
  }
  try {
    const { data } = await withTimeout(listCLIExecutions({
      workflow_session_id: sid,
      limit: 200
    }), 15000)
    const items = Array.isArray(data?.items) ? (data.items as CLIExecution[]) : []
    executions.value = items
    executionsInitialized.value = true
    syncSelectedExecution(items)
    syncExecutionStreams()
  } catch (e: any) {
    executionsInitialized.value = true
    if (!silent) {
      const isTimeout = safeText(e?.message || '').toLowerCase().includes('timeout')
      message.error(e?.response?.data?.error || (isTimeout ? '获取执行记录超时' : '获取执行记录失败'))
    }
  } finally {
    if (!silent) {
      executionsLoading.value = false
    }
  }
}

async function resume() {
  const id = safeText(props.sessionId)
  const msg = safeText(resumeMessage.value)
  if (!id || !msg || resuming.value) return
  resuming.value = true
  try {
    await postAIWorkflowMessage(id, msg)
    resumeMessage.value = ''
    await refresh(false)
    message.success('已提交，继续执行')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '提交失败')
  } finally {
    resuming.value = false
  }
}

async function pauseSession() {
  const id = safeText(session.value?.id || props.sessionId)
  if (!id || !canPauseSession.value) {
    return
  }
  pausing.value = true
  try {
    await postAIWorkflowPause(id, '用户手动暂停')
    await refresh(false)
    message.success('已请求暂停')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '暂停失败')
  } finally {
    pausing.value = false
  }
}

async function resumeSelectedExecution() {
  const executionID = safeText(selectedExecutionID.value)
  if (!executionID || !canResumeSelectedExecution.value) {
    return
  }
  resumingExecution.value = true
  try {
    const { data } = await resumeCLIExecution(executionID, { strategy: 'auto' })
    const terminalID = safeText((data as any)?.terminal_id)
    const strategy = safeText((data as any)?.strategy)
    message.success(strategy ? `已发起 CLI 会话恢复（${strategy}）` : '已发起 CLI 会话恢复')
    const sid = safeText(session.value?.id || props.sessionId)
    if (sid) {
      await Promise.all([
        loadExecutions(sid, true),
        loadWorkflowEvents(sid, true),
        loadWorkflowLogs(sid, true)
      ])
    }
    if (terminalID) {
      await router.push({ path: '/', query: { terminal: terminalID } })
    }
  } catch (e: any) {
    message.error(e?.response?.data?.error || 'CLI 会话恢复失败')
  } finally {
    resumingExecution.value = false
  }
}

watch(
  () => props.sessionId,
  async () => {
    stopExecutionStreams()
    session.value = null
    loading.value = false
    executions.value = []
    executionsInitialized.value = false
    executionsLoading.value = false
    workflowEvents.value = []
    workflowLogs.value = []
    workflowLogsInitialized.value = false
    workflowEventsLoading.value = false
    workflowLogsLoading.value = false
    workflowEventsRequestSeq++
    workflowLogsRequestSeq++
    selectedExecutionID.value = ''
    executionEvents.value = []
    executionEventsLoading.value = false
    executionEventsRequestSeq++
    payloadExpandedMap.value = {}
    workflowLogsType.value = defaultWorkflowLogsType()
    workflowLogsLimit.value = 200
    workflowLogsOrder.value = 'desc'
    workflowEventsLimit.value = 200
    workflowEventsOrder.value = 'asc'
    workflowEventsLastID.value = 0
    executionStatusFilter.value = ''
    executionRoleFilter.value = ''
    executionListLimit.value = 50
    executionEventLimit.value = 200
    executionEventOrder.value = 'asc'
    stepsStatusFilter.value = ''
    stepsLimit.value = 200
    requestSeq++
    activeTab.value = props.panelMode === 'flow' ? 'events' : 'overview'
    contextCollapsed.value = false
    showScrollToBottom.value = false
    await refresh(false)
    startPolling()
  },
  { immediate: true }
)

watch(
  () => activeTab.value,
  async (tab) => {
    if (tab === 'executions') {
      const sid = safeText(session.value?.id || props.sessionId)
      if (sid && !executionsInitialized.value && !executionsLoading.value) {
        await loadExecutions(sid, false)
      }
    }
    if (tab !== 'steps') {
      showScrollToBottom.value = false
      return
    }
    await nextTick()
    showScrollToBottom.value = !isNearBottom()
  }
)

watch(
  () => props.panelMode,
  (mode) => {
    if (mode === 'flow') {
      if (activeTab.value === 'overview') {
        activeTab.value = 'events'
      }
      if (!safeText(workflowLogsType.value)) {
        workflowLogsType.value = 'ai_native_all'
      }
      contextCollapsed.value = true
      return
    }
    if (mode === 'control') {
      contextCollapsed.value = false
      if (activeTab.value !== 'overview') {
        activeTab.value = 'overview'
      }
      return
    }
    contextCollapsed.value = false
  },
  { immediate: true }
)

watch(
  () => stepsDisplayItems.value.length,
  async (len, prev) => {
    if (!len || activeTab.value !== 'steps') return
    const shouldStick = prev === 0 || !showScrollToBottom.value || isNearBottom()
    if (shouldStick) {
      await scrollToBottom()
    } else {
      await nextTick()
      showScrollToBottom.value = !isNearBottom()
    }
  }
)

watch(
  () => [workflowLogsType.value, workflowLogsLimit.value, workflowLogsOrder.value] as const,
  async (next, prev) => {
    if (
      safeText(next[0]) === safeText(prev?.[0]) &&
      Number(next[1]) === Number(prev?.[1]) &&
      safeText(next[2]) === safeText(prev?.[2])
    ) {
      return
    }
    const sid = safeText(session.value?.id || props.sessionId)
    if (!sid) {
      return
    }
    await loadWorkflowLogs(sid, true)
  }
)

watch(
  () => workflowEventsLimit.value,
  async (next, prev) => {
    if (Number(next) === Number(prev)) {
      return
    }
    const sid = safeText(session.value?.id || props.sessionId)
    if (!sid) {
      return
    }
    await loadWorkflowEvents(sid, false)
  }
)

watch(
  () => executionEventLimit.value,
  async (next, prev) => {
    if (Number(next) === Number(prev)) {
      return
    }
    const id = safeText(selectedExecutionID.value)
    if (!id) {
      return
    }
    await loadExecutionEvents(id, true)
  }
)

watch(
  () => executionDisplayItems.value.map((row) => safeText(row.item.id)),
  (visibleIDs) => {
    const currentID = safeText(selectedExecutionID.value)
    if (!visibleIDs.length) {
      selectedExecutionID.value = ''
      executionEvents.value = []
      return
    }
    if (currentID && visibleIDs.includes(currentID)) {
      return
    }
    const nextID = safeText(visibleIDs[0])
    if (!nextID) {
      return
    }
    selectedExecutionID.value = nextID
    executionEvents.value = []
    void loadExecutionEvents(nextID, true)
  }
)

onMounted(() => {
  startPolling()
})

onUnmounted(() => {
  stopPolling()
  stopExecutionStreams()
})
</script>

<style scoped>
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

.workflow-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.workflow-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.workflow-spin {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-spin :deep(.n-spin-container) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
}

.workflow-card--external-scroll :deep(.n-card__content) {
  overflow: visible;
}

.workflow-card--external-scroll .workflow-body,
.workflow-card--external-scroll .workflow-tabs-shell,
.workflow-card--external-scroll .workflow-tabs,
.workflow-card--external-scroll .workflow-tabs :deep(.n-tabs-pane-wrapper),
.workflow-card--external-scroll .workflow-tabs :deep(.n-tabs-content),
.workflow-card--external-scroll .workflow-tabs :deep(.n-tab-pane),
.workflow-card--external-scroll .tab-pane,
.workflow-card--external-scroll .tab-layout,
.workflow-card--external-scroll .tab-split,
.workflow-card--external-scroll .tab-scroll,
.workflow-card--external-scroll .list-scroll,
.workflow-card--external-scroll .steps-container {
  overflow: visible;
}

.workflow-card--external-scroll .session-context {
  max-height: none;
  overflow: visible;
}

.session-context {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  max-height: 42%;
  overflow-y: auto;
  padding-right: 2px;
}

.session-context--collapsed {
  display: none;
}

.session-head {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px;
  background: rgba(148, 163, 184, 0.08);
}

.session-head__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.session-head__id {
  font-size: 12px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-head__meta {
  display: block;
  margin-top: 4px;
  font-size: 12px;
}

.session-head__goal {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.resume-panel {
  border: 1px dashed var(--border-color);
  border-radius: 8px;
  padding: 8px;
}

.workflow-tabs-shell {
  flex: 1;
  min-height: 200px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.workflow-tabs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-tabs :deep(.n-tabs-nav) {
  margin-bottom: 8px;
  padding-right: 2px;
}

.workflow-tabs :deep(.n-tabs-pane-wrapper) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-tabs :deep(.n-tabs-content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-tabs :deep(.n-tab-pane) {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.tab-pane {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.tab-pane--steps {
  position: relative;
}

.tab-layout {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tab-split {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
  padding-right: 2px;
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-controls {
  justify-content: flex-end;
  flex-wrap: wrap;
}

.toolbar-select {
  min-width: 98px;
}

.toolbar-select--type {
  width: 150px;
}

.toolbar-select--status {
  width: 128px;
}

.toolbar-select--order {
  width: 98px;
}

.toolbar-select--limit {
  width: 98px;
}

.tab-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.overview-stat {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px;
  background: var(--card-color);
}

.overview-stat__label {
  font-size: 12px;
  color: var(--text-color-3);
}

.overview-stat__value {
  margin-top: 4px;
  font-size: 18px;
  line-height: 1.1;
  font-weight: 600;
  color: var(--text-color-1);
}

.list-block {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.list-block--flex {
  flex: 1;
  min-height: 0;
}

.list-block--fill {
  flex: 1;
  min-height: 0;
}

.list-head-wrap {
  flex-wrap: wrap;
}

.list-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.execution-list,
.execution-event-list,
.workflow-event-list,
.workflow-log-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.execution-item {
  border-left: 2px solid var(--border-color);
  border-radius: 6px;
  padding-top: 4px;
  padding-bottom: 4px;
  cursor: pointer;
}

.execution-item--active {
  border-left-color: var(--primary-color);
  background: rgba(24, 160, 88, 0.08);
}

.execution-summary {
  display: block;
  margin-top: 4px;
  font-size: 12px;
}

.execution-event-item,
.workflow-event-item,
.workflow-log-item,
.step {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 6px;
}

.payload-pre {
  margin: 6px 0 0 0;
  padding: 6px;
  border-radius: 6px;
  background: var(--card-color);
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 220px;
  overflow-y: auto;
}

.payload-toggle {
  margin-top: 4px;
  align-self: flex-start;
}

.session-summary-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
}

.steps-container {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step__header {
  margin-bottom: 6px;
}

.step__row {
  display: grid;
  grid-template-columns: 64px 1fr;
  gap: 8px;
  align-items: start;
  margin-top: 4px;
}

.step__label {
  font-size: 12px;
  color: var(--text-color-3);
}

.step__content {
  font-size: 12px;
  color: var(--text-color-2);
}

.scroll-bottom {
  position: absolute;
  right: 8px;
  bottom: 8px;
  z-index: 2;
}

@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }

  .toolbar-select,
  .toolbar-select--type,
  .toolbar-select--status,
  .toolbar-select--order,
  .toolbar-select--limit {
    width: 100%;
    min-width: 0;
  }
}
</style>
