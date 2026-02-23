export type ISODateTimeString = string
export type UnixTimestampSeconds = number

export type OrderDirection = 'asc' | 'desc'

export interface ApiMessageResponse {
  message: string
}

export interface ApiItemResponse<T> {
  item: T
}

export interface ApiItemsResponse<T> {
  items: T[]
}

export interface ApiPaginatedResponse<T> extends ApiItemsResponse<T> {
  total: number
}

// ===== Users =====

export type UserRole = 'admin' | 'user' | 'viewer'
export type UserStatus = 'active' | 'disabled'

export interface User {
  id: string
  username: string
  email: string
  role: UserRole | string
  status: UserStatus | string
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
  last_login_at: ISODateTimeString | null
}

export interface RegisterUserRequest {
  username: string
  email: string
  password: string
  role: UserRole
  status: UserStatus
}

export interface UpdateUserRequest {
  role?: UserRole
  status?: UserStatus
}

export type ListUsersResponse = ApiItemsResponse<User>
export type UpdateUserResponse = ApiItemResponse<User>
export type RegisterUserResponse = ApiItemResponse<User>

// ===== Auth =====

export interface LoginRequest {
  username: string
  password: string
}

export interface AuthUser {
  id: string
  username: string
  role: string
}

export interface LoginResponse {
  token: string
  expires_at: UnixTimestampSeconds
  user: AuthUser
}

export interface AuthMeResponse {
  id: string
  username: string
  role: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

// ===== Runtime Version =====

export interface RuntimeVersionInfo {
  app_name: string
  version: string
  git_branch: string
  git_commit: string
  git_dirty: boolean
  build_time: string
  go_version: string
  binary_path: string
  pid: number
  started_at: ISODateTimeString
  server_host: string
  server_port: string
  server_addr: string
  static_source: string
  static_index_assets: string[]
}

export type RuntimeVersionResponse = ApiItemResponse<RuntimeVersionInfo>

// ===== Tasks =====

export type TaskStatus = 'todo' | 'in_progress' | 'paused' | 'done' | 'failed' | 'timeout' | 'archived'

export interface TaskServer {
  id: string
  name: string
}

export interface TaskProjectGroup {
  id: string
  name: string
}

export interface TaskProject {
  id: string
  name: string
  group?: TaskProjectGroup | null
}

export interface Task {
  id: string
  user_id?: string
  title: string
  description: string
  remark?: string
  status: TaskStatus | string
  priority: number
  order_index: number
  rule_set_id: string | null
  server_id?: string | null
  server?: TaskServer | null
  project_id?: string | null
  project?: TaskProject | null
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
  completed_at: ISODateTimeString | null

  // 自动化任务配置
  automation_mode?: string
  agent_session_id?: string
  target_server_ids?: string[]
  script?: string
  work_dir: string
  cli_type: string
  initial_prompt: string
  auto_start: boolean
  auto_create_dir: boolean

  // AI托管配置
  ai_managed: boolean
  ai_prompt: string
  ai_end_condition: string
  ai_error_handling: string
}

export interface ListTasksParams {
  status?: string
  priority?: string | number
  keyword?: string
  project_id?: string
  project_group_id?: string
}

export interface ListTaskHistoryParams extends ListTasksParams {
  automation_mode?: string
  limit?: number
  offset?: number
}

export type ListTasksResponse = ApiItemsResponse<Task>
export type GetTaskResponse = ApiItemResponse<Task>
export type CreateTaskResponse = ApiItemResponse<Task>
export type UpdateTaskResponse = ApiItemResponse<Task>
export type MoveTaskResponse = ApiItemResponse<Task>

export interface TasksByStatus {
  todo: Task[]
  in_progress: Task[]
  done: Task[]
  archived: Task[]
}

export interface GetTasksByStatusResponse {
  items: TasksByStatus
}

export interface CreateTaskRequest {
  title: string
  description?: string
  remark?: string
  priority?: number
  status?: TaskStatus | string
  rule_set_id?: string | null
  server_id?: string | null
  project_id?: string | null
  automation_mode?: string
  target_server_ids?: string[]
  script?: string
  work_dir?: string
  cli_type?: string
  initial_prompt?: string
  auto_start?: boolean
  auto_create_dir?: boolean
  // AI托管配置
  ai_managed?: boolean
  ai_prompt?: string
  ai_end_condition?: string
  ai_error_handling?: string
}

export interface UpdateTaskRequest {
  title?: string
  description?: string
  remark?: string
  status?: string
  priority?: number
  rule_set_id?: string | null
  server_id?: string | null
  project_id?: string | null
  automation_mode?: string
  target_server_ids?: string[]
  script?: string
  work_dir?: string
  cli_type?: string
  initial_prompt?: string
  auto_start?: boolean
  auto_create_dir?: boolean
  // AI托管配置
  ai_managed?: boolean
  ai_prompt?: string
  ai_end_condition?: string
  ai_error_handling?: string
}

export interface MoveTaskRequest {
  status: string
  order_index: number
}

export interface StartTaskResponse {
  message: string
  task: Task
  agent_session_id?: string
  execution_id?: string
  terminal_id: string
  terminal_ids?: string[]
  work_dir: string
  cli_started: boolean
  needs_user_action?: boolean
  user_action_hint?: string
}

// ===== Comments =====

export interface TaskComment {
  id: string
  task_id: string
  content: string
  author: string
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
}

export interface CreateTaskCommentRequest {
  content: string
  author: string
}

export interface UpdateCommentRequest {
  content: string
}

export type ListTaskCommentsResponse = ApiItemsResponse<TaskComment>
export type CreateTaskCommentResponse = ApiItemResponse<TaskComment>
export type UpdateCommentResponse = ApiItemResponse<TaskComment>

// ===== Terminals =====

export interface TerminalAIAssistant {
  type: string
  display_name: string
  state: string
  state_updated_at: ISODateTimeString
  detected: boolean
  version?: string
  approval_prompt?: string
}

export interface TerminalSessionMetadata {
  title: string
  pid: number
  status: string
  running_command?: string
  task_id?: string | null
  server_id?: string | null
  server_name?: string
  server_host?: string
  ai_assistant?: TerminalAIAssistant
  automation_mode?: string
  tmux_session?: string
}

// Runtime terminal (via /api/terminals) - created_at is unix seconds
export interface Terminal {
  id: string
  title: string
  task_id: string | null
  status: string
  hidden?: boolean
  pid: number
  metadata: TerminalSessionMetadata
  created_at: UnixTimestampSeconds
}

export interface CreateTerminalRequest {
  server_id: string
  title?: string
  task_id?: string
}

export interface RecoverTerminalRequest {
  mode?: 'auto' | 'resume' | 'continue'
  title?: string
}

export interface RecoverTerminalResponse {
  message: string
  action: 'resumed' | 'continued' | 'resume_unavailable' | string
  source_terminal_id: string
  item?: Terminal
}

export type ListTerminalsResponse = ApiItemsResponse<Terminal>
export type GetTerminalResponse = ApiItemResponse<Terminal>
export type CreateTerminalResponse = ApiItemResponse<Terminal>

export interface TerminalStatsResponse {
  total: number
}

// Database terminal session (via /api/tasks/:id/detail, /api/tasks/:id/terminals, etc.)
export interface TerminalSession {
  id: string
  user_id?: string
  title: string
  task_id: string | null
  shell: string
  status: string
  pid: number
  tmux_session: string
  rule_mode: string
  rule_set_id: string | null
  created_at: ISODateTimeString
  closed_at: ISODateTimeString | null
  task?: Task
}

// Terminal logs
export type LogType = 'input' | 'output' | 'system' | string

export interface LogEntry {
  id: string
  terminal_id: string | null
  task_id: string | null
  log_type: LogType
  content: string
  created_at: ISODateTimeString
}

export interface TerminalLogsParams {
  limit?: number
  offset?: number
  type?: string
  source?: 'all' | 'native' | 'pty' | string
  include_raw?: boolean
  order?: OrderDirection
}

export interface TerminalLogsResponse extends ApiPaginatedResponse<LogEntry> {
  order: OrderDirection
}

export type ClearTerminalLogsResponse = ApiMessageResponse
export type DeleteTerminalLogResponse = ApiMessageResponse

// WebSocket messages (/api/terminal/ws)
export type TerminalWsClientMessage =
  | { type: 'input'; data: string }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'close' }

export interface TerminalWsApprovalEvent {
  action: string
  input: string
  reasoning: string
  confidence: number
  rule_matched: string
  ai_decision: boolean
  auto_handled: boolean
}

export interface TerminalWsAILogEvent {
  type: string
  message: string
  input_type?: string
  input_data?: string
}

export type TerminalWsServerMessage =
  | { type: 'ready'; metadata: TerminalSessionMetadata }
  | { type: 'data'; data: string }
  | { type: 'metadata'; metadata: TerminalSessionMetadata }
  | { type: 'exit'; exit_code?: number; message?: string }
  | { type: 'approval'; approval_result?: TerminalWsApprovalEvent; message?: string }
  | { type: 'ai_log'; ai_log?: TerminalWsAILogEvent }
  | { type: 'message'; message: string }
  | { type: 'approval_needed'; terminal_id: string; prompt_type?: string; prompt_content?: string }
  | { type: 'error'; message: string }

export type TerminalWsMessage = TerminalWsClientMessage | TerminalWsServerMessage

// ===== Logs (admin pages) =====

export interface LogSessionInfo {
  terminal_id: string
  title: string
  log_count: number
  first_log: ISODateTimeString
  last_log: ISODateTimeString
}

export interface LogListParams {
  terminal_id?: string
  type?: string
  keyword?: string
  limit?: number
  offset?: number
}

export type ListLogsResponse = ApiPaginatedResponse<LogEntry>
export type ListLogSessionsResponse = ApiItemsResponse<LogSessionInfo>
export type DeleteLogResponse = ApiMessageResponse

export interface LogExportParams {
  format: 'json' | 'csv'
  start_date: string
  end_date: string
  terminal_id?: string
}

// ===== RuleSets / Automation =====

export type RuleSetType = 'system' | 'task' | 'terminal'
export type ApprovalMode = 'manual' | 'auto_yes' | 'smart'
export type AutoInputType = 'yes' | 'y' | 'enter' | 'option1'

export interface RuleSet {
  id: string
  name: string
  type: RuleSetType | string
  approval_mode: ApprovalMode | string
  auto_input_type: AutoInputType | string
  whitelist_patterns: string
  blacklist_patterns: string
  ai_provider_id: string | null
  ai_prompt: string
  context_lines: number
  detect_claude_code: boolean
  detect_codex: boolean
  detect_gemini: boolean
  notify_on_block: boolean
  notify_on_approve: boolean
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
}

export interface RuleSetRequest {
  name?: string
  approval_mode?: string
  auto_input_type?: string
  whitelist_patterns?: string[]
  blacklist_patterns?: string[]
  ai_provider_id?: string | null
  ai_prompt?: string
  context_lines?: number
  detect_claude_code?: boolean
  detect_codex?: boolean
  detect_gemini?: boolean
  notify_on_block?: boolean
  notify_on_approve?: boolean
}

export type GetSystemRuleResponse = ApiItemResponse<RuleSet>
export type UpdateSystemRuleResponse = ApiItemResponse<RuleSet> & ApiMessageResponse
export type ListRuleSetsResponse = ApiItemsResponse<RuleSet>
export type GetRuleSetResponse = ApiItemResponse<RuleSet>
export type CreateRuleSetResponse = ApiItemResponse<RuleSet> & ApiMessageResponse
export type DeleteRuleSetResponse = ApiMessageResponse

export interface RuleSetImportResult {
  message: string
  created: number
  updated: number
  total: number
}

export type RuleSetsImportRequest =
  | { items?: RuleSet[]; rule_sets?: RuleSet[] }
  | RuleSet[]

export interface TerminalRuleModeUpdateRequest {
  rule_mode: string
  rule_set_id?: string | null
}

export interface TerminalRuleModeResponse {
  rule_mode: string
  rule_set_id: string | null
  rule_set: RuleSet | null
  effective_rule_set: RuleSet | null
  task_id: string | null
}

export interface DefaultPatternsResponse {
  whitelist: string[]
  blacklist: string[]
}

export interface AIProviderConfig {
  id: string
  name: string
  provider: string
  base_url: string
  api_key: string
  model: string
  temperature: number
  max_tokens: number
  is_default: boolean
  enabled: boolean
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
}

export interface AIProviderConfigRequest {
  name: string
  provider: string
  base_url?: string
  api_key?: string
  model: string
  temperature?: number
  max_tokens?: number
  is_default?: boolean
  enabled?: boolean
}

export type ListAIProvidersResponse = ApiItemsResponse<AIProviderConfig>
export type GetAIProviderResponse = ApiItemResponse<AIProviderConfig>
export type CreateAIProviderResponse = ApiMessageResponse & {
  item: { id: string; name: string }
}
export type UpdateAIProviderResponse = ApiMessageResponse
export type DeleteAIProviderResponse = ApiMessageResponse

export type AutomationMessageType = 'approval_needed' | 'blocked' | 'info' | 'warning' | 'error' | string
export type AutomationMessageStatus = 'unread' | 'read' | 'handled' | 'dismissed' | string

export interface AutomationMessage {
  id: string
  terminal_id: string | null
  task_id: string | null
  type: AutomationMessageType
  title: string
  content: string
  context: string
  status: AutomationMessageStatus
  action_taken: string
  priority: number
  expires_at: ISODateTimeString | null
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
  read_at: ISODateTimeString | null
  handled_at: ISODateTimeString | null
}

export interface ListMessagesParams {
  status?: string
  type?: string
  terminal_id?: string
  limit?: number
  offset?: number
}

export type ListMessagesResponse = ApiPaginatedResponse<AutomationMessage>
export type GetMessageResponse = ApiItemResponse<AutomationMessage>
export interface UnreadCountResponse {
  count: number
}

export interface HandleMessageRequest {
  action: string
}

export interface ApprovalRecord {
  id: string
  terminal_id: string
  ai_session_id: string | null
  prompt_type: string
  prompt_content: string
  response: string
  auto_approved: boolean
  auto_handled?: boolean
  rule_matched: string
  ai_decision: string
  created_at: ISODateTimeString
}

export interface ListApprovalRecordsParams {
  terminal_id?: string
  limit?: number
  offset?: number
}

export type ListApprovalRecordsResponse = ApiPaginatedResponse<ApprovalRecord>

export interface LoginRecord {
  id: string
  user_id: string | null
  identifier: string
  username: string
  success: boolean
  error: string
  ip: string
  user_agent: string
  created_at: ISODateTimeString
}

export interface ListLoginRecordsParams {
  keyword?: string
  user_id?: string
  success?: string
  limit?: number
  offset?: number
}

export type ListLoginRecordsResponse = ApiPaginatedResponse<LoginRecord>

// ===== Secrets =====

export type SecretType = 'ssh_password' | 'ssh_key' | 'api_key' | string

export interface Secret {
  id: string
  name: string
  type: SecretType
  meta: string
  created_at: ISODateTimeString
  updated_at: ISODateTimeString
}

export interface CreateSecretRequest {
  name: string
  type: SecretType
  plaintext: string
  meta: string
}

export interface UpdateSecretRequest {
  name?: string
  type?: SecretType
  plaintext?: string
  meta?: string
}

export type ListSecretsResponse = ApiItemsResponse<Secret>
export type CreateSecretResponse = ApiItemResponse<Secret>
export type UpdateSecretResponse = ApiItemResponse<Secret>
export type DeleteSecretResponse = ApiMessageResponse

// ===== Agent Config =====

export type AIAgentType = 'claude-code' | 'codex' | 'gemini' | 'copilot' | 'cursor'

export interface AgentConfig {
  agent_type: AIAgentType
  display_name: string
  enabled: boolean
  priority: number
  detect_modes: string[]
}

export type GetAgentConfigsResponse = ApiItemsResponse<AgentConfig>
export type UpdateAgentConfigsRequest = { items: AgentConfig[] }

// ===== CLI Executions =====

export type CLIExecutionRole = 'primary' | 'review' | 'replay' | 'audit' | string
export type CLIExecutionStatus = 'running' | 'completed' | 'error' | 'timeout' | 'cancelled' | string

export interface CLIExecution {
  id: string
  task_id: string | null
  terminal_id: string | null
  workflow_run_id: string | null
  workflow_session_id: string | null
  parent_execution_id: string | null
  role: CLIExecutionRole
  tool: string
  mode: string
  source: string
  prompt_preview: string
  status: CLIExecutionStatus
  exit_code?: number | null
  error_message: string
  metadata: string
  started_at: ISODateTimeString
  completed_at: ISODateTimeString | null
  updated_at: ISODateTimeString
}

export interface CLIExecutionEvent {
  seq: number
  event_type: string
  payload: any
  created_at: ISODateTimeString
}

export interface ListCLIExecutionsParams {
  status?: string
  task_id?: string
  workflow_session_id?: string
  parent_id?: string
  role?: string
  mode?: string
  source?: string
  tool?: string
  limit?: number
}

export interface ListCLIExecutionsResponse {
  items: CLIExecution[]
  count: number
  status?: string
  task_id?: string
  workflow_session_id?: string
  parent_id?: string
  role?: string
  mode?: string
  source?: string
  tool?: string
}

export type GetCLIExecutionResponse = ApiItemResponse<CLIExecution>

export interface ListCLIExecutionEventsParams {
  after?: number
  limit?: number
}

export interface ListCLIExecutionEventsResponse {
  items: CLIExecutionEvent[]
  count: number
  after: number
  hasMore: boolean
}

export interface ListCLIExecutionChildrenResponse {
  items: CLIExecution[]
  count: number
  parent_id: string
}

export interface ResumeCLIExecutionRequest {
  strategy?: 'auto' | 'native' | 'prompt_concat' | string
  session_id?: string
  work_dir?: string
  server_id?: string
  cli_type?: string
  prompt?: string
  title?: string
}

export interface ResumeCLIExecutionResponse {
  message: string
  strategy: string
  execution_id?: string
  parent_execution_id: string
  terminal_id: string
  command: string
  session_id?: string
  server_id?: string
  work_dir?: string
}

export interface TaskHistoryWorkflowSession {
  id: string
  status: string
  workflow_id: string
  started_at: ISODateTimeString
  completed_at: ISODateTimeString | null
}

export interface TaskHistoryItem {
  task: Task
  workflow_session?: TaskHistoryWorkflowSession | null
  latest_execution?: CLIExecution | null
}

export interface TaskHistoryStatsOverview {
  total: number
  by_status: Record<string, number>
  by_mode: Record<string, number>
}

export interface TaskHistoryStatsGroupItem {
  group_id: string
  group_name: string
  total: number
  by_status: Record<string, number>
  by_mode: Record<string, number>
}

export interface TaskHistoryStatsProjectItem {
  project_id: string
  project_name: string
  group_id: string
  group_name: string
  total: number
  by_status: Record<string, number>
  by_mode: Record<string, number>
}

export interface TaskHistoryStats {
  overview: TaskHistoryStatsOverview
  by_group: TaskHistoryStatsGroupItem[]
  by_project: TaskHistoryStatsProjectItem[]
}

export interface ListTaskHistoryResponse {
  items: TaskHistoryItem[]
  count: number
  total: number
  limit: number
  offset: number
  project_id?: string
  project_group_id?: string
  status?: string
  automation_mode?: string
  keyword?: string
  stats?: TaskHistoryStats
}

export type CLIExecutionSSEEventType = 'ready' | 'message' | 'done' | 'timeout' | 'error' | 'unknown'

export interface CLIExecutionSSEEnvelope {
  event: CLIExecutionSSEEventType
  data: any
}
