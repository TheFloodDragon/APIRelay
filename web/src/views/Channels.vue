<script setup>
import { computed, getCurrentInstance, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import { resetBreakerAndConfirm } from '../breakerReset'
import { confirmAction } from '../composables/useConfirm'
import { DEFAULT_HEALTH_CONFIG, hasHealth, healthTotal, healthText, healthTitle, healthClass as healthClassBy } from '../health'
import Modal from '../components/Modal.vue'
import Drawer from '../components/Drawer.vue'
import PageState from '../components/PageState.vue'
import PageHeader from '../components/PageHeader.vue'
import ConsoleSection from '../components/ConsoleSection.vue'
import DataToolbar from '../components/DataToolbar.vue'
import InlineNotice from '../components/InlineNotice.vue'
import ConsoleIcon from '../components/ConsoleIcon.vue'
import HeaderOverrideEditor from '../components/HeaderOverrideEditor.vue'
import BodyOverrideEditor from '../components/BodyOverrideEditor.vue'
import ChannelConsoleHeader from '../components/ChannelConsoleHeader.vue'

const { proxy } = getCurrentInstance()
const route = useRoute()
const router = useRouter()
const notify = (message, type = 'info', duration) => proxy?.$toast?.add(message, type, duration)

const channels = ref([])
const channelTypes = ref([])
const protocols = ref([])
const loading = ref(true)
const loadError = ref('')
const metadataLoading = ref(false)
const selectedChannelId = ref(null)
const editorBaseline = ref('')

const showEditor = ref(false)
const probing = ref(false)
const saving = ref(false)
const editorError = ref('')
const editorTab = ref('connection')
const revealKey = ref(false)
const headerValidation = ref({ valid: true, error: '', allowedCount: 0, ignored: [] })
const bodyValidation = ref({ valid: true, error: '', keyCount: 0, ignored: [] })
const models = ref([])
const rules = ref([])
const newModelName = ref('')
const testing = ref({})
const testResults = ref({})

const batchTesting = ref(false)
const batchDone = ref(0)
const batchTotal = ref(0)
const batchSummary = ref(null)

// 模型行的勾选集合。存 _uid 而非模型名：名称可以被编辑成空或重名，
// 用它作标识会在改名途中丢失选择。
const selectedModelUids = ref(new Set())
// 批量设价/设协议的输入值。
const bulkPriceInput = ref('')
const bulkPriceOutput = ref('')
const bulkProtocol = ref('')

const checkupLoadingId = ref(null)
const showCheckup = ref(false)
const checkupChannelName = ref('')
const checkupResults = ref([])
const checkupSummary = ref(null)

const selectedIds = ref(new Set())
const globalTestPrompt = ref("Say 'hi' in one word.")
const healthConfig = ref({ ...DEFAULT_HEALTH_CONFIG })
const bulkDeleting = ref(false)

const togglingIds = ref(new Set())
const deletingIds = ref(new Set())
const resettingIds = ref(new Set())
const dragIndex = ref(null)
const dropIndex = ref(null)
const reordering = ref(false)
const channelQuery = ref('')
const statusFilter = ref('all')
// 响应式时钟，供 breakerState 判断冷却是否已过期（见 syncCooldownClock）。
const nowMs = ref(Date.now())

const blank = () => ({
  id: 0,
  name: '',
  type: 1,
  base_url: '',
  // key 只用于提交新凭据；后端不再回显明文，留空表示沿用已保存的值。
  key: '',
  key_masked: '',
  has_key: false,
  group: 'default',
  header_override: '',
  body_override: '',
  test_prompt: '',
  priority: 0,
  weight: 1,
  status: 1,
})
const form = ref(blank())

// 凭据是否满足要求：已保存过（has_key）或本次表单填写了新值。
const hasCredential = computed(() => Boolean(form.value.has_key || String(form.value.key || '').trim()))

const sortedChannels = computed(() => {
  const query = channelQuery.value.trim().toLowerCase()
  return channels.value.filter((channel) => {
    const state = breakerState(channel)
    if (statusFilter.value !== 'all' && state !== statusFilter.value) return false
    if (!query) return true
    const haystack = [channel.name, channel.group, channel.base_url, typeName(channel.type), ...(channel._models || []).map((item) => item.name)]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    return haystack.includes(query)
  })
})
const canReorder = computed(() => statusFilter.value === 'all' && !channelQuery.value.trim())
const allVisibleSelected = computed(() => sortedChannels.value.length > 0
  && sortedChannels.value.every((channel) => selectedIds.value.has(channel.id)))
const channelSummary = computed(() => channels.value.reduce((summary, channel) => {
  const state = breakerState(channel)
  summary.total += 1
  summary[state] = (summary[state] || 0) + 1
  return summary
}, { total: 0, run: 0, trip: 0, off: 0, test: 0 }))
const routeSegments = computed(() => [
  { key: 'run', label: '运行', count: channelSummary.value.run, tone: 'run' },
  { key: 'test', label: '检查', count: channelSummary.value.test, tone: 'test' },
  { key: 'trip', label: '熔断', count: channelSummary.value.trip, tone: 'trip' },
  { key: 'off', label: '停用', count: channelSummary.value.off, tone: 'off' },
].map((item) => ({
  ...item,
  percent: channelSummary.value.total ? Math.round((item.count / channelSummary.value.total) * 100) : 0,
})))
const enabledCount = computed(() => models.value.filter((model) => model.enabled && model.name.trim()).length)

// ---- 模型批量选择 ----

const selectedModels = computed(() => models.value.filter((model) => selectedModelUids.value.has(model._uid)))
const selectedModelCount = computed(() => selectedModels.value.length)
const allModelsSelected = computed(() => models.value.length > 0 && selectedModelCount.value === models.value.length)
// 部分选中时表头复选框显示 indeterminate，比"未选中"更准确地反映状态。
const someModelsSelected = computed(() => selectedModelCount.value > 0 && !allModelsSelected.value)
// 已选模型中有多少是启用的，用于决定"启用/停用"按钮的语义。
const selectedEnabledCount = computed(() => selectedModels.value.filter((model) => model.enabled).length)
const hasModelTesting = computed(() => Object.values(testing.value).some(Boolean))
const editorBusy = computed(() => saving.value || probing.value || batchTesting.value || hasModelTesting.value)
// 批量操作在测试/保存进行中禁用，避免改动正在被使用的数据。
const bulkModelActionsDisabled = computed(() => batchTesting.value || saving.value || hasModelTesting.value)
const canSave = computed(() => Boolean(
  form.value.name.trim()
  && form.value.base_url.trim()
  && hasCredential.value
  && enabledCount.value > 0
  && headerValidation.value.valid
  && bodyValidation.value.valid
))
const saveHint = computed(() => {
  if (!form.value.name.trim()) return '填写渠道名称后可保存'
  if (!form.value.base_url.trim()) return '填写 Base URL 后可保存'
  if (!hasCredential.value) return '填写 API Key 后可保存'
  if (!enabledCount.value) return '至少启用一个模型后可保存'
  if (!headerValidation.value.valid) return '修正请求头配置后可保存'
  if (!bodyValidation.value.valid) return '修正请求体配置后可保存'
  return '配置完整，可以保存'
})
const editorSteps = computed(() => ({
  connection: Boolean(form.value.name.trim() && form.value.base_url.trim() && hasCredential.value),
  models: enabledCount.value > 0,
  overrides: headerValidation.value.valid && bodyValidation.value.valid,
  reliability: true,
}))
const editorSections = computed(() => [
  { key: 'connection', index: '01', label: '连接与身份', note: editorSteps.value.connection ? '连接信息完整' : '需要完善凭据', icon: 'key' },
  { key: 'models', index: '02', label: '模型与价格', note: `${enabledCount.value} 个模型已启用`, icon: 'models' },
  { key: 'overrides', index: '03', label: '请求改写', note: editorSteps.value.overrides ? 'JSON 校验通过' : '存在配置错误', icon: 'command' },
  { key: 'reliability', index: '04', label: '可靠性', note: form.value.status === 1 ? '渠道参与路由' : '渠道已停用', icon: 'shield' },
])
const activeEditor = computed(() => editorSections.value.find((item) => item.key === editorTab.value) || editorSections.value[0])
const customHeaderCount = computed(() => headerValidation.value.valid ? headerValidation.value.allowedCount : 0)
const checkupRate = computed(() => {
  const summary = checkupSummary.value
  if (!summary?.total) return 0
  return Math.round((summary.success / summary.total) * 100)
})
const testRecordRows = computed(() => models.value
  .map((model) => ({ model, result: testResults.value[model.name] }))
  .filter((row) => row.result || testing.value[row.model.name]))
// 用 computed 缓存快照，避免 isDirty 与依赖它的模板节点在每次按键时重复序列化整个模型数组。
const currentSnapshot = computed(() => JSON.stringify({ form: form.value, models: models.value, rules: rules.value }))
const isDirty = computed(() => showEditor.value && editorBaseline.value !== currentSnapshot.value)
const saveStatus = computed(() => {
  if (saving.value) return { label: '保存中', tone: 'saving' }
  if (isDirty.value) return { label: '有未保存更改', tone: 'dirty' }
  if (form.value.id) return { label: '已保存 · 自动保存关闭', tone: 'saved' }
  return { label: '新渠道 · 自动保存关闭', tone: 'idle' }
})
const selectedChannel = computed(() => channels.value.find((channel) => channel.id === selectedChannelId.value) || null)
const editorTitle = computed(() => (form.value.id ? `渠道配置 · ${form.value.name || '未命名渠道'}` : '新建渠道'))

function editorSnapshot() {
  return currentSnapshot.value
}

function markEditorBaseline() {
  editorBaseline.value = editorSnapshot()
}

function typeName(type) {
  const found = channelTypes.value.find((item) => item.value === type)
  return found ? found.name : String(type)
}

function displayEndpoint(value) {
  const raw = String(value || '').trim()
  if (!raw) return '未配置地址'
  try {
    const url = new URL(raw)
    return `${url.host}${url.pathname === '/' ? '' : url.pathname}`
  } catch {
    return raw.replace(/^https?:\/\//, '')
  }
}

function modelCount(channel) {
  if (Array.isArray(channel._models)) return channel._models.length
  if (channel.model_configs) {
    try {
      const list = JSON.parse(channel.model_configs)
      if (Array.isArray(list)) return list.length
    } catch {
      // 兼容旧 models 字段。
    }
  }
  return (channel.models || '').split(',').map((item) => item.trim()).filter(Boolean).length
}

function modelHealth(channel, item) {
  return channel?.model_health?.[item?.name] || null
}

// 依据当前设置阈值返回健康 chip class。
function healthClass(health) {
  return healthClassBy(health, healthConfig.value)
}

function channelHealth(channel) {
  const stats = Object.values(channel?.model_health || {}).filter(Boolean)
  const called = stats.filter(hasHealth)
  if (!called.length) return null
  const total = called.reduce((sum, item) => sum + healthTotal(item), 0)
  const success = called.reduce((sum, item) => sum + (Number(item.success) || 0), 0)
  const failed = called.reduce((sum, item) => sum + (Number(item.failed) || 0), 0)
  return { total, success, failed, availability: total ? (success / total) * 100 : 0 }
}

function breakerState(channel) {
  if (checkupLoadingId.value === channel.id) return 'test'
  if (channel.status !== 1) return 'off'
  // 读响应式时钟而非 Date.now()：后者不是响应式源，冷却到点后
  // 「已熔断」标记会一直挂着，直到用户手动刷新或触发其它响应式变更。
  if (channel.cooldown_until && channel.cooldown_until > nowMs.value) return 'trip'
  return 'run'
}

function breakerText(channel) {
  return { run: '运行中', test: '测试中', trip: '已熔断', off: '已停用' }[breakerState(channel)]
}

function validateHeaders(action) {
  if (!headerValidation.value.valid) {
    editorTab.value = 'overrides'
    editorError.value = `无法${action}：${headerValidation.value.error}`
    notify(editorError.value, 'warn')
    return false
  }
  if (!bodyValidation.value.valid) {
    editorTab.value = 'overrides'
    editorError.value = `无法${action}：${bodyValidation.value.error}`
    notify(editorError.value, 'warn')
    return false
  }
  return true
}

function updateSet(target, value, active) {
  const next = new Set(target.value)
  if (active) next.add(value)
  else next.delete(value)
  target.value = next
}

// load 会被 save / toggleChannel / removeChannel / resetBreaker / 刷新按钮多路并发调用。
// 没有守卫时后发起的请求若先返回，旧响应会覆盖新列表，乐观更新过的 status 也会被
// 回滚成过期值。用自增序号只接受最新一次请求的结果。
let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  loading.value = true
  loadError.value = ''
  try {
    const data = (await api.get('/channels')) || []
    if (seq !== loadSeq) return // 已有更新的请求在飞，丢弃本次结果
    channels.value = data.map((channel) => ({ ...channel, _models: parseModels(channel) }))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => channels.value.some((channel) => channel.id === id)))
    const stillListed = channels.value.some((channel) => channel.id === selectedChannelId.value)
    if (!stillListed && !showEditor.value) selectedChannelId.value = null
    nowMs.value = Date.now()
    syncCooldownClock()
  } catch (error) {
    if (seq !== loadSeq) return // 过期请求的错误不应覆盖当前状态
    loadError.value = error.message || '渠道清单加载失败'
    notify(`加载失败：${loadError.value}`, 'error')
  } finally {
    // 仅最新请求负责收起 loading，避免过期请求提前结束加载态。
    if (seq === loadSeq) loading.value = false
  }
}

async function loadMeta() {
  if (metadataLoading.value) return
  metadataLoading.value = true
  try {
    const [types, protocolList, promptData, healthData] = await Promise.all([
      api.get('/channel-types'),
      api.get('/protocols'),
      api.get('/settings/test-prompt'),
      api.get('/settings/model-health'),
    ])
    channelTypes.value = types || []
    protocols.value = protocolList || []
    globalTestPrompt.value = promptData?.prompt || globalTestPrompt.value
    healthConfig.value = { ...healthConfig.value, ...(healthData || {}) }
  } catch (error) {
    notify(`加载元数据失败：${error.message}`, 'error')
  } finally {
    metadataLoading.value = false
  }
}

// 行级稳定标识。用数组 index 作 v-for key 时，addModel 的头部插入会让所有既有行的
// key 整体位移，Vue 复用 DOM 时把上一行的输入状态（焦点、IME 组合中的文本）留在错误的
// 模型上；removeModel 的 splice 同理。_uid 只存在于前端，不参与提交 payload。
let modelRowSeq = 0
function nextRowUid() {
  modelRowSeq += 1
  return `m${modelRowSeq}`
}

function newModelRow(overrides = {}) {
  return {
    _uid: nextRowUid(),
    name: '',
    enabled: true,
    protocol: '',
    upstream: '',
    input: 0,
    output: 0,
    ...overrides,
  }
}

function parseModels(channel) {
  if (channel.model_configs) {
    try {
      const list = JSON.parse(channel.model_configs)
      if (Array.isArray(list)) {
        return list.map((model) => newModelRow({
          name: model.name || '',
          enabled: model.enabled !== false,
          protocol: model.protocol || '',
          upstream: model.upstream || '',
          input: model.input || 0,
          output: model.output || 0,
        }))
      }
    } catch {
      // 兼容旧 models 字段。
    }
  }
  return (channel.models || '').split(',').map((item) => item.trim()).filter(Boolean)
    .map((name) => newModelRow({ name }))
}

function parseRules(channel) {
  if (channel.protocol_rules) {
    try {
      const list = JSON.parse(channel.protocol_rules)
      if (Array.isArray(list)) {
        return list.map((rule) => ({
          pattern: rule.pattern || '',
          protocol: rule.protocol || 'anthropic',
        }))
      }
    } catch {
      // 无效旧值按空规则处理。
    }
  }
  return []
}

function resetEditorState() {
  editorError.value = ''
  editorTab.value = 'connection'
  revealKey.value = false
  newModelName.value = ''
  testing.value = {}
  testResults.value = {}
  batchSummary.value = null
  batchDone.value = 0
  batchTotal.value = 0
  // 切换渠道时清空模型勾选与批量输入，否则会带到下一个渠道的模型列表上。
  selectedModelUids.value = new Set()
  bulkPriceInput.value = ''
  bulkPriceOutput.value = ''
  bulkProtocol.value = ''
}

async function confirmDiscardChanges() {
  if (!isDirty.value) return true
  return confirmAction({
    title: '放弃未保存更改',
    message: '当前渠道配置尚未保存。继续后这些更改会丢失。',
    confirmLabel: '放弃更改',
  })
}

async function openCreate() {
  if (!(await confirmDiscardChanges())) return
  selectedChannelId.value = null
  form.value = blank()
  models.value = []
  rules.value = []
  resetEditorState()
  const type = channelTypes.value.find((item) => item.value === form.value.type)
  if (type) form.value.base_url = type.default_base_url
  showEditor.value = true
  markEditorBaseline()
}

async function openEdit(channel) {
  if (!channel) return
  if (!(await confirmDiscardChanges())) return
  selectedChannelId.value = channel.id
  // 后端不回显明文凭据：key 始终以空值进入表单，留空提交即表示沿用已保存的值。
  form.value = { ...blank(), ...channel, key: '' }
  models.value = (Array.isArray(channel._models) ? channel._models : parseModels(channel)).map((item) => ({ ...item }))
  rules.value = parseRules(channel)
  resetEditorState()
  showEditor.value = true
  markEditorBaseline()
}

async function closeEditor() {
  if (editorBusy.value || !(await confirmDiscardChanges())) return
  editorBaseline.value = editorSnapshot()
  showEditor.value = false
}

function onTypeChange() {
  const type = channelTypes.value.find((item) => item.value === form.value.type)
  if (type && !form.value.base_url) form.value.base_url = type.default_base_url
}

function addModel() {
  const name = newModelName.value.trim()
  if (!name) return
  if (models.value.some((model) => model.name === name)) {
    notify('模型已存在', 'warn')
    return
  }
  models.value.unshift(newModelRow({ name }))
  newModelName.value = ''
}

function removeModel(index, model) {
  if (testing.value[model?.name] || batchTesting.value) return
  models.value.splice(index, 1)
  if (model?._uid) dropModelSelection([model._uid])
  if (model?.name) forgetModelTestState([model.name])
}

// ---- 模型批量操作 ----
//
// 模型是编辑器内的本地状态，批量操作只改内存数组，随渠道一起保存才生效。
// 这与渠道列表页的批量删除不同（那个立即落库），因此 UI 上要明确提示"需保存"。

function forgetModelTestState(names) {
  const nextResults = { ...testResults.value }
  const nextTesting = { ...testing.value }
  names.forEach((name) => {
    if (!name) return
    delete nextResults[name]
    delete nextTesting[name]
  })
  testResults.value = nextResults
  testing.value = nextTesting
}

function dropModelSelection(uids) {
  const next = new Set(selectedModelUids.value)
  uids.forEach((uid) => next.delete(uid))
  selectedModelUids.value = next
}

function toggleModelSelected(uid) {
  const next = new Set(selectedModelUids.value)
  if (next.has(uid)) next.delete(uid)
  else next.add(uid)
  selectedModelUids.value = next
}

function toggleSelectAllModels() {
  if (allModelsSelected.value) {
    selectedModelUids.value = new Set()
    return
  }
  selectedModelUids.value = new Set(models.value.map((model) => model._uid))
}

function clearModelSelection() {
  selectedModelUids.value = new Set()
}

// 只保留仍存在于列表中的选择项。探测模型或切换渠道后列表会变，
// 残留的 uid 会让"已选 N 项"与实际不符。
function pruneModelSelection() {
  const alive = new Set(models.value.map((model) => model._uid))
  const next = new Set()
  selectedModelUids.value.forEach((uid) => {
    if (alive.has(uid)) next.add(uid)
  })
  selectedModelUids.value = next
}

function bulkSetModelEnabled(enabled) {
  if (bulkModelActionsDisabled.value) return
  const targets = selectedModels.value
  if (!targets.length) return
  targets.forEach((model) => { model.enabled = enabled })
  notify(`已${enabled ? '启用' : '停用'} ${targets.length} 个模型，保存后生效`, 'success')
}

async function bulkRemoveModels() {
  if (bulkModelActionsDisabled.value) return
  const targets = selectedModels.value
  if (!targets.length) return

  const confirmed = await confirmAction({
    title: '批量移除模型',
    message: `确认从当前渠道移除选中的 ${targets.length} 个模型？\n\n改动在保存渠道后生效。`,
    confirmLabel: `移除 ${targets.length} 个模型`,
  })
  if (!confirmed) return

  const removedUids = new Set(targets.map((model) => model._uid))
  const removedNames = targets.map((model) => model.name).filter(Boolean)
  models.value = models.value.filter((model) => !removedUids.has(model._uid))
  clearModelSelection()
  forgetModelTestState(removedNames)
  notify(`已移除 ${targets.length} 个模型，保存后生效`, 'success')
}

function bulkSetModelProtocol() {
  if (bulkModelActionsDisabled.value) return
  const targets = selectedModels.value
  if (!targets.length) return
  const protocol = bulkProtocol.value
  targets.forEach((model) => { model.protocol = protocol })
  const label = protocol
    ? (protocols.value.find((item) => item.value === protocol)?.name || protocol)
    : '继承规则'
  notify(`已将 ${targets.length} 个模型的协议设为「${label}」，保存后生效`, 'success')
}

function bulkSetModelPrice() {
  if (bulkModelActionsDisabled.value) return
  const targets = selectedModels.value
  if (!targets.length) return

  // 两个价格框都留空视为无操作，避免误把价格清零。
  const rawInput = bulkPriceInput.value.trim()
  const rawOutput = bulkPriceOutput.value.trim()
  if (!rawInput && !rawOutput) {
    notify('请先填写要应用的输入价或输出价', 'warn')
    return
  }

  const parsed = {}
  for (const [field, raw] of [['input', rawInput], ['output', rawOutput]]) {
    if (!raw) continue
    const value = Number(raw)
    if (!Number.isFinite(value) || value < 0) {
      notify(`${field === 'input' ? '输入价' : '输出价'}必须是非负数`, 'warn')
      return
    }
    parsed[field] = value
  }

  targets.forEach((model) => {
    if (parsed.input !== undefined) model.input = parsed.input
    if (parsed.output !== undefined) model.output = parsed.output
  })
  const parts = []
  if (parsed.input !== undefined) parts.push(`输入价 ${parsed.input}`)
  if (parsed.output !== undefined) parts.push(`输出价 ${parsed.output}`)
  notify(`已将 ${targets.length} 个模型的${parts.join('、')}更新，保存后生效`, 'success')
}

function testPayload() {
  return {
    // 带上 id 让后端在 key 留空时回退到已保存的凭据（编辑态不再持有明文）。
    id: form.value.id || 0,
    type: form.value.type,
    base_url: form.value.base_url,
    key: String(form.value.key || '').trim(),
    group: form.value.group || 'default',
    protocol_rules: JSON.stringify(rules.value.filter((rule) => rule.pattern.trim() && rule.protocol)),
    header_override: form.value.header_override || '',
    body_override: form.value.body_override || '',
    test_prompt: form.value.test_prompt || '',
    prompt: form.value.test_prompt?.trim() || globalTestPrompt.value,
  }
}

async function testModel(model) {
  const name = model.name.trim()
  if (!name || testing.value[name] || batchTesting.value || saving.value) return
  if (!validateHeaders('测试模型')) return
  if (!form.value.base_url) {
    notify('请先填写 Base URL', 'warn')
    return
  }
  if (!hasCredential.value) {
    notify('请先填写 API Key', 'warn')
    return
  }

  testing.value = { ...testing.value, [name]: true }
  testResults.value = {
    ...testResults.value,
    [name]: { model: name, pending: true, success: false },
  }
  try {
    const result = await api.post('/channels/test', {
      ...testPayload(),
      model_configs: JSON.stringify([{
        name,
        enabled: true,
        protocol: model.protocol || '',
        upstream: model.upstream || '',
      }]),
      model: name,
    })
    testResults.value = { ...testResults.value, [name]: { ...result, model: name } }
    notify(result.success ? `模型 ${name} 连通正常` : `模型 ${name} 测试失败`, result.success ? 'success' : 'error')
  } catch (error) {
    testResults.value = {
      ...testResults.value,
      [name]: { model: name, success: false, error: error.message || '请求失败' },
    }
    notify(`测试失败：${error.message || '请求失败'}`, 'error')
  } finally {
    testing.value = { ...testing.value, [name]: false }
  }
}

// runBatchTest 执行一次批量测试。targets 为待测模型行，label 用于提示文案。
// 「测试全部启用」与「测试所选」共用它，避免两套并行状态与结果合并逻辑。
async function runBatchTest(targets, label) {
  if (batchTesting.value || saving.value || probing.value || hasModelTesting.value) return
  if (!validateHeaders(label)) return
  if (!form.value.base_url) {
    notify('请先填写 Base URL', 'warn')
    return
  }
  if (!hasCredential.value) {
    notify('请先填写 API Key', 'warn')
    return
  }
  const testable = targets.filter((model) => model.name.trim())
  if (!testable.length) {
    notify('没有可测试的模型', 'warn')
    return
  }

  batchTesting.value = true
  batchTotal.value = testable.length
  batchDone.value = 0
  batchSummary.value = null
  const pending = { ...testResults.value }
  testable.forEach((model) => {
    pending[model.name.trim()] = { model: model.name.trim(), pending: true, success: false }
  })
  testResults.value = pending

  try {
    const response = await api.post('/channels/test-batch', {
      ...testPayload(),
      model_configs: JSON.stringify(testable.map((model) => ({
        name: model.name.trim(),
        enabled: true,
        protocol: model.protocol || '',
        upstream: model.upstream || '',
      }))),
      models: testable.map((model) => model.name.trim()),
    })
    const results = response.results || []
    const merged = { ...testResults.value }
    results.forEach((result) => {
      if (result?.model) merged[result.model] = result
    })
    testResults.value = merged
    batchDone.value = results.length
    batchSummary.value = response.summary || null
    if (response.summary) {
      const { success, failed } = response.summary
      notify(
        failed === 0 ? `全部 ${success} 个模型连通正常` : `测试完成：成功 ${success} · 失败 ${failed}`,
        failed === 0 ? 'success' : 'warn',
      )
    }
  } catch (error) {
    testable.forEach((model) => {
      const name = model.name.trim()
      if (testResults.value[name]?.pending) {
        testResults.value[name] = { model: name, success: false, error: error.message || '请求失败' }
      }
    })
    testResults.value = { ...testResults.value }
    notify(`批量测试失败：${error.message || '请求失败'}`, 'error')
  } finally {
    batchTesting.value = false
  }
}

async function testAllInModal() {
  const enabled = models.value.filter((model) => model.enabled && model.name.trim())
  if (!enabled.length) {
    notify('没有可测试的启用模型', 'warn')
    return
  }
  await runBatchTest(enabled, '批量测试')
}

// 测试所选：不要求模型处于启用状态 —— 用户可能正是想先验证再决定启不启用。
async function bulkTestSelectedModels() {
  const targets = selectedModels.value
  if (!targets.length) return
  await runBatchTest(targets, '测试所选模型')
}

async function checkupChannel(channel) {
  if (checkupLoadingId.value !== null) return
  checkupLoadingId.value = channel.id
  checkupChannelName.value = channel.name
  checkupResults.value = []
  checkupSummary.value = null
  try {
    const response = await api.post(`/channels/${channel.id}/test-all`, {
      prompt: channel.test_prompt?.trim() || globalTestPrompt.value,
    })
    checkupResults.value = response.results || []
    checkupSummary.value = response.summary || null
    showCheckup.value = true
    if (response.summary) {
      const { success, failed } = response.summary
      notify(
        failed === 0
          ? `「${channel.name}」全部 ${success} 个模型连通正常`
          : `「${channel.name}」体检：成功 ${success} · 失败 ${failed}`,
        failed === 0 ? 'success' : 'warn',
      )
    }
  } catch (error) {
    notify(`体检失败：${error.message || '请求失败'}`, 'error')
  } finally {
    checkupLoadingId.value = null
  }
}

async function fetchModels() {
  if (probing.value || saving.value || batchTesting.value || hasModelTesting.value) return
  if (!validateHeaders('探测模型')) return
  if (!form.value.base_url || !hasCredential.value) {
    editorError.value = '请先填写 Base URL 和 API Key'
    return
  }
  editorError.value = ''
  probing.value = true
  try {
    const data = await api.post('/channels/probe-models', {
      id: form.value.id || 0,
      type: form.value.type,
      base_url: form.value.base_url,
      key: String(form.value.key || '').trim(),
      header_override: form.value.header_override || '',
    })
    const fetched = data.models || []
    const existing = new Set(models.value.map((model) => model.name))
    let added = 0
    fetched.forEach((name) => {
      if (!existing.has(name)) {
        models.value.push(newModelRow({ name }))
        existing.add(name)
        added += 1
      }
    })
    notify(`探测到 ${fetched.length} 个模型，新增 ${added} 个`, 'success')
  } catch (error) {
    editorError.value = `模型探测失败：${error.message || '网络错误'}`
    notify(editorError.value, 'error')
  } finally {
    probing.value = false
  }
}

function cleanPayload() {
  const cleanModels = models.value.filter((model) => model.name.trim()).map((model) => ({
    name: model.name.trim(),
    enabled: Boolean(model.enabled),
    protocol: model.protocol || '',
    upstream: model.upstream || '',
    input: Number(model.input) || 0,
    output: Number(model.output) || 0,
  }))
  const cleanRules = rules.value.filter((rule) => rule.pattern.trim() && rule.protocol)
  const payload = {
    ...form.value,
    name: form.value.name.trim(),
    base_url: form.value.base_url.trim(),
    group: form.value.group.trim() || 'default',
    weight: Math.max(1, Number(form.value.weight) || 1),
    model_configs: JSON.stringify(cleanModels),
    protocol_rules: JSON.stringify(cleanRules),
    models: cleanModels.filter((model) => model.enabled).map((model) => model.name).join(','),
  }
  // 只读展示字段不回传。
  delete payload.key_masked
  delete payload.has_key
  // 凭据留空表示沿用已保存的值，此时不发送该字段，避免误传空串。
  const key = String(form.value.key || '').trim()
  if (key) payload.key = key
  else delete payload.key
  return payload
}

async function save() {
  if (saving.value || probing.value || batchTesting.value || hasModelTesting.value) return
  if (!validateHeaders('保存')) return
  if (!canSave.value) {
    editorError.value = '请填写渠道名称、Base URL、API Key，并至少启用一个模型'
    return
  }
  editorError.value = ''
  saving.value = true
  try {
    const payload = cleanPayload()
    const originalId = form.value.id
    let response
    if (originalId) {
      response = await api.put(`/channels/${originalId}`, payload)
      notify('渠道已更新', 'success')
    } else {
      response = await api.post('/channels', payload)
      notify('渠道已创建', 'success')
    }
    editorBaseline.value = editorSnapshot()
    selectedChannelId.value = response?.id || originalId || null
    showEditor.value = false
    await load()
    const savedChannel = channels.value.find((channel) => channel.id === selectedChannelId.value)
      || channels.value.find((channel) => channel.name === payload.name)
    if (savedChannel) selectedChannelId.value = savedChannel.id
  } catch (error) {
    editorError.value = error.message || '保存失败'
    notify(editorError.value, 'error')
  } finally {
    saving.value = false
  }
}

function toggleSelected(channelId) {
  const next = new Set(selectedIds.value)
  if (next.has(channelId)) next.delete(channelId)
  else next.add(channelId)
  selectedIds.value = next
}

function toggleSelectAllVisible() {
  const next = new Set(selectedIds.value)
  const shouldClear = allVisibleSelected.value
  sortedChannels.value.forEach((channel) => {
    if (shouldClear) next.delete(channel.id)
    else next.add(channel.id)
  })
  selectedIds.value = next
}

async function bulkDeleteChannels() {
  const ids = [...selectedIds.value]
  if (!ids.length || bulkDeleting.value) return
  const confirmed = await confirmAction({
    title: '批量删除渠道',
    message: `确认删除选中的 ${ids.length} 个渠道？\n\n渠道及其模型路由将一并删除，此操作不可撤销。`,
    confirmLabel: `删除 ${ids.length} 个渠道`,
  })
  if (!confirmed) return
  bulkDeleting.value = true
  try {
    await api.post('/channels/bulk-delete', { ids })
    selectedIds.value = new Set()
    notify(`已删除 ${ids.length} 个渠道`, 'success')
    await load()
  } catch (error) {
    notify(`批量删除失败：${error.message}`, 'error')
  } finally {
    bulkDeleting.value = false
  }
}

async function toggleChannel(channel) {
  if (togglingIds.value.has(channel.id)) return
  const previous = channel.status
  const nextStatus = previous === 1 ? 2 : 1
  const syncEditor = form.value.id === channel.id
  const editorWasDirty = isDirty.value
  updateSet(togglingIds, channel.id, true)
  channel.status = nextStatus
  if (syncEditor) form.value.status = nextStatus
  try {
    await api.patch(`/channels/${channel.id}/status`, { enabled: nextStatus === 1 })
    if (syncEditor && !editorWasDirty) markEditorBaseline()
    notify(`「${channel.name}」已${nextStatus === 1 ? '启用' : '停用'}`, 'success')
  } catch (error) {
    channel.status = previous
    if (syncEditor) form.value.status = previous
    notify(`状态切换失败：${error.message}`, 'error')
  } finally {
    updateSet(togglingIds, channel.id, false)
  }
}

async function removeChannel(channel) {
  if (deletingIds.value.has(channel.id)) return
  const confirmed = await confirmAction({
    title: '删除渠道',
    message: `确认删除渠道「${channel.name}」？\n\n此操作不可撤销。`,
    confirmLabel: '删除渠道',
  })
  if (!confirmed) return
  updateSet(deletingIds, channel.id, true)
  try {
    await api.delete(`/channels/${channel.id}`)
    notify('渠道已删除', 'success')
    await load()
  } catch (error) {
    notify(`删除失败：${error.message}`, 'error')
  } finally {
    updateSet(deletingIds, channel.id, false)
  }
}

async function resetBreaker(channel) {
  if (resettingIds.value.has(channel.id)) return
  const wasTripped = breakerState(channel) === 'trip'
  const editorWasDirty = isDirty.value
  updateSet(resettingIds, channel.id, true)
  try {
    const { channel: refreshed } = await resetBreakerAndConfirm(api, channel.id, async () => {
      await load()
      return channels.value
    })
    if (form.value.id === channel.id) {
      form.value.cooldown_until = refreshed.cooldown_until || 0
      if (!editorWasDirty) markEditorBaseline()
    }
    notify(wasTripped ? `「${channel.name}」已解除熔断` : `「${channel.name}」健康状态已重置`, 'success')
  } catch (error) {
    notify(`健康状态重置失败：${error.message}`, 'error')
  } finally {
    updateSet(resettingIds, channel.id, false)
  }
}

function onDragStart(index, event) {
  if (reordering.value || !canReorder.value) {
    event.preventDefault()
    return
  }
  dragIndex.value = index
  dropIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index) {
  if (!reordering.value) dropIndex.value = index
}

function onDrop(index) {
  if (reordering.value) return
  const from = dragIndex.value
  if (from === null || from === index) {
    onDragEnd()
    return
  }
  const previous = channels.value.slice()
  const reordered = channels.value.slice()
  const [moved] = reordered.splice(from, 1)
  reordered.splice(index, 0, moved)
  channels.value = reordered
  onDragEnd()
  persistOrder(previous)
}

function onDragEnd() {
  dragIndex.value = null
  dropIndex.value = null
}

async function persistOrder(previous) {
  if (reordering.value) return
  reordering.value = true
  try {
    const ids = channels.value.map((channel) => channel.id)
    await api.post('/channels/reorder', { ids })
    const top = channels.value.length - 1
    channels.value.forEach((channel, index) => {
      channel.priority = top - index
    })
    notify('渠道顺序已更新', 'success')
  } catch (error) {
    channels.value = previous
    notify(`排序保存失败，已回滚：${error.message}`, 'error')
  } finally {
    reordering.value = false
  }
}

// 冷却状态的响应式时钟。低频推进（5s）即可：冷却是分钟级的，
// 用户感知不到几秒误差，但必须让 chip 和筛选计数在到点后自动恢复。
let clockTimer = null

function syncCooldownClock() {
  const hasCooldown = channels.value.some(
    (channel) => channel.cooldown_until && channel.cooldown_until > nowMs.value,
  )
  if (hasCooldown && clockTimer === null) {
    clockTimer = window.setInterval(() => {
      nowMs.value = Date.now()
      // 冷却全部结束后停表，避免页面常驻空转的定时器。
      if (!channels.value.some((channel) => channel.cooldown_until && channel.cooldown_until > nowMs.value)) {
        stopCooldownClock()
      }
    }, 5000)
  } else if (!hasCooldown) {
    stopCooldownClock()
  }
}

function stopCooldownClock() {
  if (clockTimer !== null) {
    window.clearInterval(clockTimer)
    clockTimer = null
  }
}

onMounted(async () => {
  await Promise.all([loadMeta(), load()])
  if (route.query.action === 'new') {
    await openCreate()
    const query = { ...route.query }
    delete query.action
    router.replace({ query })
  }
})

onBeforeUnmount(stopCooldownClock)

// 暴露 load 供测试直接验证并发竞态守卫（多条业务路径都会调用它），
// 以及模型批量操作所依赖的本地状态。
defineExpose({ load, models, selectedModelUids, bulkPriceInput, bulkPriceOutput, bulkProtocol })
</script>

<template>
  <div class="page-workbench channels-page min-w-0">
    <PageHeader eyebrow="上游路由" title="上游渠道" description="在同一工作台中编排渠道优先级，并维护连接、模型、请求改写与可靠性策略。">
      <template #actions>
        <button type="button" class="btn" :disabled="loading" aria-label="刷新渠道列表" @click="load">
          <ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': loading }" />
          {{ loading ? '刷新中' : '刷新' }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="metadataLoading" aria-label="新建渠道" @click="openCreate">
          <ConsoleIcon name="plus" class="h-4 w-4" />新建渠道
        </button>
      </template>
    </PageHeader>

    <section class="sheet channel-console min-w-0" aria-label="渠道队列">
      <ChannelConsoleHeader
        v-model:query="channelQuery" v-model:status="statusFilter"
        :summary="channelSummary" :segments="routeSegments"
        :selected-count="selectedIds.size" :bulk-deleting="bulkDeleting"
        :reordering="reordering" :visible-count="sortedChannels.length"
        @bulk-delete="bulkDeleteChannels"
      />
      <div class="channel-list-scroll">
        <PageState
          :loading="loading" :error="loadError" :empty="!channels.length"
          empty-text="暂无渠道" empty-hint="创建第一个上游渠道后即可开始承接模型请求。"
          @retry="load"
        >
          <template #empty>
            <button type="button" class="btn btn-primary" @click="openCreate">
              <ConsoleIcon name="plus" class="h-4 w-4" />新建渠道
            </button>
          </template>
          <div class="hidden min-w-0 overflow-x-auto lg:block">
            <table class="table-eng channel-table" aria-label="上游渠道队列">
              <thead>
                <tr>
                  <th class="w-[3.5rem]">
                    <input
                      type="checkbox" class="align-middle" :checked="allVisibleSelected"
                      :disabled="!sortedChannels.length" aria-label="全选当前列表中的渠道"
                      @change="toggleSelectAllVisible"
                    />
                  </th>
                  <th class="w-[4.5rem]">优先级</th>
                  <th>渠道 / 上游地址</th>
                  <th class="w-[11%]">分组 · 协议</th>
                  <th class="w-[8%]">状态</th>
                  <th class="w-[11%]">健康</th>
                  <th class="w-[6%] text-right">模型</th>
                  <th class="w-[6%] text-right">权重</th>
                  <th class="w-[12rem] text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(channel, index) in sortedChannels" :key="channel.id"
                  class="channel-row cursor-pointer"
                  :class="[
                    selectedChannelId === channel.id ? 'channel-row-selected' : '',
                    channel.status !== 1 ? 'channel-row-off' : '',
                    dragIndex === index ? 'channel-row-dragging' : '',
                    dropIndex === index && dragIndex !== null && dragIndex !== index ? 'channel-row-dropzone' : '',
                  ]"
                  :draggable="!reordering && canReorder" tabindex="0"
                  :aria-label="`编辑渠道 ${channel.name}`"
                  @click="openEdit(channel)" @keydown.enter.prevent="openEdit(channel)"
                  @dragstart="onDragStart(index, $event)" @dragover.prevent="onDragOver(index)"
                  @drop.prevent="onDrop(index)" @dragend="onDragEnd"
                >
                  <td @click.stop>
                    <div class="flex items-center gap-1">
                      <input
                        type="checkbox" :checked="selectedIds.has(channel.id)"
                        :aria-label="`选择渠道 ${channel.name}`" @change="toggleSelected(channel.id)"
                      />
                      <button
                        type="button" class="channel-grip" :disabled="reordering || !canReorder"
                        :aria-label="`拖动调整 ${channel.name} 的优先级`"
                        :title="canReorder ? '拖动整行调整优先级' : '清除搜索与筛选后可排序'"
                      ><ConsoleIcon name="bars" class="h-4 w-4" /></button>
                    </div>
                  </td>
                  <td class="num !text-ink">{{ String(index + 1).padStart(2, '0') }}</td>
                  <td>
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="channel-state-dot" :class="`channel-state-${breakerState(channel)}`" :title="breakerText(channel)" aria-hidden="true"><i></i></span>
                      <span class="truncate text-[13px] font-semibold text-ink" :title="channel.name">{{ channel.name }}</span>
                    </div>
                    <div class="mt-1 truncate pl-4 font-mono text-[10px] text-soft" :title="channel.base_url">{{ displayEndpoint(channel.base_url) }}</div>
                  </td>
                  <td>
                    <div class="truncate text-xs text-ink">{{ channel.group || 'default' }}</div>
                    <div class="mt-1 truncate font-mono text-[10px] text-faint">{{ typeName(channel.type) }}</div>
                  </td>
                  <td>
                    <span class="chip" :class="breakerState(channel) === 'run' ? 'chip-run' : breakerState(channel) === 'trip' ? 'chip-trip' : breakerState(channel) === 'off' ? '' : 'chip-test'">{{ breakerText(channel) }}</span>
                  </td>
                  <td>
                    <span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">{{ healthText(channelHealth(channel)) }}</span>
                  </td>
                  <td class="num !text-ink">{{ modelCount(channel) }}</td>
                  <td class="num !text-ink">×{{ channel.weight }}</td>
                  <td @click.stop>
                    <div class="flex items-center justify-end gap-1">
                      <button type="button" class="icon-btn h-8 w-8" :disabled="checkupLoadingId !== null" :aria-label="`检查渠道 ${channel.name}`" :title="checkupLoadingId === channel.id ? '检查中' : '运行全模型检查'" @click="checkupChannel(channel)"><ConsoleIcon name="bolt" class="h-4 w-4" :class="{ 'animate-pulse': checkupLoadingId === channel.id }" /></button>
                      <button v-if="breakerState(channel) === 'trip'" type="button" class="icon-btn h-8 w-8" :disabled="resettingIds.has(channel.id)" :aria-label="`解除渠道 ${channel.name} 的熔断`" title="解除熔断" @click="resetBreaker(channel)"><ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': resettingIds.has(channel.id) }" /></button>
                      <button type="button" class="icon-btn h-8 w-8" :disabled="deletingIds.has(channel.id)" :aria-label="`删除渠道 ${channel.name}`" title="删除渠道" @click="removeChannel(channel)"><ConsoleIcon name="trash" class="h-4 w-4" /></button>
                      <button type="button" class="channel-switch mx-1" :class="{ 'channel-switch-on': channel.status === 1 }" :disabled="togglingIds.has(channel.id)" :aria-pressed="channel.status === 1" :aria-label="`${channel.status === 1 ? '停用' : '启用'}渠道 ${channel.name}`" @click="toggleChannel(channel)"><span aria-hidden="true"></span></button>
                      <button type="button" class="btn btn-sm" @click="openEdit(channel)">配置</button>
                    </div>
                  </td>

                </tr>
              </tbody>
            </table>
          </div>
          <div class="channel-card-list lg:hidden">
            <article
              v-for="(channel, index) in sortedChannels" :key="channel.id"
              class="channel-card min-w-0 cursor-pointer border-l-2 px-3 py-3"
              :class="[
                selectedChannelId === channel.id ? 'channel-card-selected' : 'border-l-transparent',
                channel.status !== 1 ? 'channel-row-off' : '',
              ]"
              tabindex="0" :aria-label="`编辑渠道 ${channel.name}`"
              @click="openEdit(channel)" @keydown.enter.prevent="openEdit(channel)"
            >
              <div class="flex min-w-0 items-start gap-2.5">
                <input type="checkbox" class="mt-1 shrink-0" :checked="selectedIds.has(channel.id)" :aria-label="`选择渠道 ${channel.name}`" @click.stop @change="toggleSelected(channel.id)" />
                <div class="min-w-0 flex-1">
                  <div class="flex min-w-0 items-center gap-2">
                    <span class="channel-state-dot" :class="`channel-state-${breakerState(channel)}`" :title="breakerText(channel)" aria-hidden="true"><i></i></span>
                    <span class="min-w-0 flex-1 truncate text-sm font-semibold text-ink" :title="channel.name">{{ channel.name }}</span>
                    <span class="font-mono text-[9px] text-faint">{{ String(index + 1).padStart(2, '0') }}</span>
                  </div>
                  <div class="mt-1 truncate font-mono text-[10px] text-soft" :title="channel.base_url">{{ displayEndpoint(channel.base_url) }}</div>
                  <div class="mt-2 flex min-w-0 flex-wrap items-center gap-1.5">
                    <span class="chip" :class="breakerState(channel) === 'run' ? 'chip-run' : breakerState(channel) === 'trip' ? 'chip-trip' : breakerState(channel) === 'off' ? '' : 'chip-test'">{{ breakerText(channel) }}</span>
                    <span class="chip">{{ modelCount(channel) }} 模型</span>
                    <span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">{{ healthText(channelHealth(channel)) }}</span>
                  </div>
                </div>
              </div>
              <div class="mt-3 flex min-w-0 items-center justify-between gap-2 border-t border-line/70 pt-2.5">
                <div class="min-w-0 truncate text-[10px] text-soft">{{ channel.group || 'default' }} · {{ typeName(channel.type) }} · 权重 ×{{ channel.weight }}</div>
                <div class="flex shrink-0 items-center gap-1">
                  <button type="button" class="icon-btn h-8 w-8" :disabled="checkupLoadingId !== null" :aria-label="`检查渠道 ${channel.name}`" @click.stop="checkupChannel(channel)"><ConsoleIcon name="bolt" class="h-4 w-4" :class="{ 'animate-pulse': checkupLoadingId === channel.id }" /></button>
                  <button type="button" class="channel-switch" :class="{ 'channel-switch-on': channel.status === 1 }" :disabled="togglingIds.has(channel.id)" :aria-pressed="channel.status === 1" :aria-label="`${channel.status === 1 ? '停用' : '启用'}渠道 ${channel.name}`" @click.stop="toggleChannel(channel)"><span aria-hidden="true"></span></button>
                  <ConsoleIcon name="chevronRight" class="h-4 w-4 text-faint" />
                </div>
              </div>
            </article>
          </div>


          <div v-if="channels.length && !sortedChannels.length" class="m-3 rounded-lg border border-dashed border-line bg-surface px-4 py-10 text-center">
            <div class="font-medium text-ink">没有匹配的渠道</div><p class="mt-1 text-xs text-soft">尝试清空搜索词或切换运行状态。</p>
            <button type="button" class="btn btn-sm mt-3" @click="channelQuery = ''; statusFilter = 'all'">清除筛选</button>
          </div>
        </PageState>
      </div>
    </section>

    <Drawer :open="showEditor" :title="editorTitle" width="max-w-6xl" :persistent="editorBusy" @close="closeEditor">
      <div class="channel-detail min-w-0">
        <header class="channel-detail-meta">
          <div class="min-w-0">
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <span class="channel-state-dot" :class="form.status === 1 ? 'channel-state-run' : 'channel-state-off'" aria-hidden="true"><i></i></span>
              <span class="min-w-0 truncate text-sm font-semibold text-ink">{{ form.name || '未命名渠道' }}</span>
              <span class="chip">{{ form.id ? `ID ${form.id}` : '新建' }}</span>
            </div>
            <p class="mt-1 truncate font-mono text-[10px] text-soft">{{ displayEndpoint(form.base_url) }}</p>
          </div>
          <div class="hidden shrink-0 items-center gap-2 sm:flex"><span class="chip chip-blue">{{ typeName(form.type) }}</span><span class="chip">{{ enabledCount }} / {{ models.length }} 模型</span></div>
        </header>

        <nav class="detail-mobile-nav" role="tablist" aria-label="渠道配置区域">
          <button v-for="section in editorSections" :key="`mobile-${section.key}`" type="button" role="tab" :aria-selected="editorTab === section.key" @click="editorTab = section.key"><ConsoleIcon :name="section.icon" class="h-4 w-4" /><span>{{ section.label }}</span></button>
        </nav>

        <div class="channel-detail-layout">
          <aside class="channel-detail-nav" aria-label="渠道配置导航">
            <div class="px-3 pb-2 pt-3 font-mono text-[9px] uppercase tracking-[.14em] text-faint">配置区域</div>
            <nav class="space-y-1" role="tablist">
              <button v-for="section in editorSections" :key="section.key" type="button" role="tab" :aria-selected="editorTab === section.key" @click="editorTab = section.key">
                <ConsoleIcon :name="section.icon" class="h-4 w-4 shrink-0" />
                <span class="min-w-0 flex-1"><b>{{ section.label }}</b><small>{{ section.note }}</small></span>
                <span class="detail-nav-state" :class="editorSteps[section.key] ? 'is-done' : 'is-pending'"></span>
              </button>
            </nav>
            <dl class="detail-summary"><div><dt>协议</dt><dd>{{ typeName(form.type) }}</dd></div><div><dt>模型</dt><dd>{{ enabledCount }} / {{ models.length }}</dd></div><div><dt>权重</dt><dd>×{{ form.weight || 1 }}</dd></div><div><dt>请求头</dt><dd>{{ customHeaderCount }}</dd></div></dl>
          </aside>

          <main class="channel-detail-content">
            <div class="mb-3 flex min-w-0 items-start justify-between gap-3">
              <div class="min-w-0"><div class="flex items-center gap-2 font-mono text-[9px] uppercase tracking-[.12em] text-blue-grid"><span>{{ activeEditor.index }}</span><span>{{ activeEditor.label }}</span></div><p class="mt-1 text-xs text-soft">{{ activeEditor.note }}</p></div>
              <span class="chip shrink-0" :class="editorSteps[editorTab] ? 'chip-run' : 'chip-test'">{{ editorSteps[editorTab] ? '区域就绪' : '待完善' }}</span>
            </div>
            <InlineNotice v-if="editorError" class="mb-3" tone="danger" title="无法完成操作">{{ editorError }}</InlineNotice>

            <div v-show="editorTab === 'connection'" class="space-y-3">
              <ConsoleSection title="连接与身份" description="定义渠道名称、默认协议、上游地址与凭据。" eyebrow="Connection">
                <div class="grid min-w-0 gap-4 md:grid-cols-2">
                  <div><label class="field-label" for="channel-name">渠道名称 *</label><input id="channel-name" v-model="form.name" class="input" placeholder="例：OpenAI 主账号" autocomplete="off" data-autofocus /></div>
                  <div><label class="field-label" for="channel-group">分组</label><input id="channel-group" v-model="form.group" class="input input-mono" placeholder="default" autocomplete="off" /></div>
                  <div><label class="field-label" for="channel-type">默认协议 *</label><select id="channel-type" v-model.number="form.type" class="input" @change="onTypeChange"><option v-for="type in channelTypes" :key="type.value" :value="type.value">{{ type.name }}</option></select></div>
                  <div class="md:col-span-2"><label class="field-label" for="channel-url">Base URL *</label><input id="channel-url" v-model="form.base_url" class="input input-mono" placeholder="https://api.openai.com" autocomplete="off" /></div>
                  <div class="md:col-span-2">
                    <label class="field-label" for="channel-key">API Key {{ form.has_key ? '' : '*' }}</label>
                    <div class="channel-key-row">
                      <input id="channel-key" v-model="form.key" :type="revealKey ? 'text' : 'password'" class="input input-mono min-w-0" :placeholder="form.has_key ? '留空表示不修改' : 'upstream-key'" name="apirelay-upstream-key" autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false" data-1p-ignore data-lpignore="true" data-form-type="other" aria-describedby="channel-key-hint" />
                      <button type="button" class="btn shrink-0" :aria-pressed="revealKey" :aria-label="revealKey ? '隐藏 API Key' : '显示 API Key'" @click="revealKey = !revealKey"><ConsoleIcon :name="revealKey ? 'x' : 'key'" class="h-4 w-4" />{{ revealKey ? '隐藏' : '显示' }}</button>
                    </div>
                    <p id="channel-key-hint" class="field-hint">
                      <template v-if="form.has_key">已保存凭据 <code class="input-mono">{{ form.key_masked }}</code>，留空则保持不变；填入新值将覆盖。</template>
                      <template v-else>上游厂商密钥。保存后不再回显明文，仅显示掩码。</template>
                    </p>
                  </div>
                </div>
              </ConsoleSection>
            </div>

            <div v-show="editorTab === 'models'" class="space-y-3">
              <ConsoleSection title="模型与价格" :description="`${enabledCount} 个启用，共 ${models.length} 个配置；价格单位为 USD / 1M tokens。`" eyebrow="Models" flush>
                <template #actions>
                  <button type="button" class="btn btn-sm" :disabled="editorBusy || !form.base_url || !hasCredential || enabledCount === 0" @click="testAllInModal"><ConsoleIcon name="bolt" class="h-4 w-4" />{{ batchTesting ? `批测中 ${batchDone}/${batchTotal}` : '批量测试' }}</button>
                  <button type="button" class="btn btn-sm" :disabled="editorBusy || !form.base_url || !hasCredential" @click="fetchModels"><ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': probing }" />{{ probing ? '探测中' : '探测模型' }}</button>
                </template>
                <div class="p-3 sm:p-4">
                  <InlineNotice v-if="batchTesting || batchSummary" class="mb-3" :tone="batchSummary?.failed ? 'warning' : 'info'" title="批量测试"><span v-if="batchTesting">执行中 {{ batchDone }} / {{ batchTotal }}</span><span v-else-if="batchSummary">通过 {{ batchSummary.success }}，失败 {{ batchSummary.failed }}，总计 {{ batchSummary.total }}。</span></InlineNotice>
                  <DataToolbar label="添加模型">
                    <input v-model="newModelName" class="input input-mono min-w-0 flex-1" placeholder="模型显示名（可使用 * 通配）" aria-label="新模型名称" @keyup.enter="addModel" />
                    <template #actions><button type="button" class="btn btn-primary btn-sm" @click="addModel"><ConsoleIcon name="plus" class="h-4 w-4" />添加模型</button></template>
                  </DataToolbar>

                  <!-- 批量操作条：仅在有勾选时出现，避免常态占用空间。
                       所有操作都只改本地状态，因此明确标注"保存后生效"。 -->
                  <div v-if="selectedModelCount" class="model-bulk-bar mt-3" role="group" aria-label="模型批量操作">
                    <div class="model-bulk-head">
                      <span class="chip chip-blue shrink-0">已选 {{ selectedModelCount }} / {{ models.length }}</span>
                      <span class="min-w-0 flex-1 text-[11px] text-soft">批量改动为本地编辑，需保存渠道后生效。</span>
                      <button type="button" class="btn btn-sm shrink-0" @click="clearModelSelection">清空选择</button>
                    </div>

                    <div class="model-bulk-actions">
                      <button type="button" class="btn btn-sm" :disabled="bulkModelActionsDisabled || selectedEnabledCount === selectedModelCount" @click="bulkSetModelEnabled(true)">
                        <ConsoleIcon name="checkCircle" class="h-4 w-4" />启用所选
                      </button>
                      <button type="button" class="btn btn-sm" :disabled="bulkModelActionsDisabled || selectedEnabledCount === 0" @click="bulkSetModelEnabled(false)">
                        <ConsoleIcon name="x" class="h-4 w-4" />停用所选
                      </button>
                      <button type="button" class="btn btn-sm" :disabled="bulkModelActionsDisabled || !form.base_url || !hasCredential" @click="bulkTestSelectedModels">
                        <ConsoleIcon name="bolt" class="h-4 w-4" />{{ batchTesting ? `测试中 ${batchDone}/${batchTotal}` : '测试所选' }}
                      </button>
                      <button type="button" class="btn btn-danger btn-sm" :disabled="bulkModelActionsDisabled" @click="bulkRemoveModels">
                        <ConsoleIcon name="trash" class="h-4 w-4" />移除所选
                      </button>
                    </div>

                    <div class="model-bulk-fields">
                      <label class="model-bulk-field">
                        <span class="field-label">统一协议</span>
                        <div class="flex min-w-0 gap-2">
                          <select v-model="bulkProtocol" class="input min-w-0 flex-1 py-1 text-[12px]" aria-label="批量设置协议">
                            <option value="">继承规则</option>
                            <option v-for="protocol in protocols" :key="`bulk-${protocol.value}`" :value="protocol.value">{{ protocol.name }}</option>
                          </select>
                          <button type="button" class="btn btn-sm shrink-0" data-bulk-apply="protocol" aria-label="将协议应用到所选模型" :disabled="bulkModelActionsDisabled" @click="bulkSetModelProtocol">应用</button>
                        </div>
                      </label>
                      <label class="model-bulk-field">
                        <span class="field-label">统一价格（USD / 1M tokens）</span>
                        <div class="flex min-w-0 gap-2">
                          <input v-model="bulkPriceInput" type="number" step="0.01" min="0" class="input min-w-0 flex-1 py-1 font-mono text-[12px]" placeholder="输入价" aria-label="批量输入价" />
                          <input v-model="bulkPriceOutput" type="number" step="0.01" min="0" class="input min-w-0 flex-1 py-1 font-mono text-[12px]" placeholder="输出价" aria-label="批量输出价" />
                          <button type="button" class="btn btn-sm shrink-0" data-bulk-apply="price" aria-label="将价格应用到所选模型" :disabled="bulkModelActionsDisabled" @click="bulkSetModelPrice">应用</button>
                        </div>
                      </label>
                    </div>
                  </div>
                  <div v-if="models.length" class="model-table-wrap mt-3 hidden lg:block">
                    <table class="table-eng min-w-[780px]" aria-label="模型配置表">
                      <thead><tr>
                        <th class="w-10">
                          <input
                            type="checkbox"
                            :checked="allModelsSelected"
                            :indeterminate="someModelsSelected"
                            :aria-label="allModelsSelected ? '取消全选模型' : '全选模型'"
                            @change="toggleSelectAllModels"
                          />
                        </th>
                        <th class="w-16">启用</th><th>模型名称</th><th class="w-36">协议</th><th>上游映射</th><th class="w-24 text-right">输入价</th><th class="w-24 text-right">输出价</th><th class="w-20 text-right">测试</th><th class="w-20 text-right">删除</th>
                      </tr></thead>
                      <tbody><tr v-for="(model, index) in models" :key="model._uid" :class="{ 'model-row-selected': selectedModelUids.has(model._uid) }">
                        <td>
                          <input
                            type="checkbox"
                            :checked="selectedModelUids.has(model._uid)"
                            :aria-label="`选择模型 ${model.name || index + 1}`"
                            @change="toggleModelSelected(model._uid)"
                          />
                        </td>
                        <td><button type="button" class="channel-switch" :class="{ 'channel-switch-on': model.enabled }" :aria-pressed="model.enabled" @click="model.enabled = !model.enabled"><span></span></button></td>
                        <td><input v-model="model.name" class="input input-mono py-1 text-[12px]" placeholder="显示名" /></td>
                        <td><select v-model="model.protocol" class="input py-1 text-[12px]"><option value="">继承规则</option><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select></td>
                        <td><input v-model="model.upstream" class="input input-mono py-1 text-[12px]" placeholder="留空则同显示名" /></td>
                        <td><input v-model.number="model.input" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" /></td>
                        <td><input v-model.number="model.output" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" /></td>
                        <td class="text-right"><button type="button" class="btn btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim()" @click="testModel(model)">{{ testing[model.name] ? '测试中' : '单测' }}</button></td>
                        <td class="text-right"><button type="button" class="btn btn-danger btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting" @click="removeModel(index, model)">删除</button></td>
                      </tr></tbody>
                    </table>
                  </div>
                  <div v-if="models.length" class="mt-3 grid gap-3 lg:hidden">
                    <!-- 移动端没有表头，需要独立的全选入口，否则无法开始勾选 -->
                    <label class="flex items-center gap-2 rounded-lg border border-line bg-surface px-3 py-2 text-[12px]">
                      <input
                        type="checkbox"
                        class="shrink-0"
                        :checked="allModelsSelected"
                        :indeterminate="someModelsSelected"
                        :aria-label="allModelsSelected ? '取消全选模型' : '全选模型'"
                        @change="toggleSelectAllModels"
                      />
                      <span class="min-w-0 flex-1">{{ selectedModelCount ? `已选 ${selectedModelCount} / ${models.length}` : '全选模型' }}</span>
                    </label>
                    <article v-for="(model, index) in models" :key="model._uid" class="model-card rounded-lg border border-line bg-surface p-3" :class="{ 'model-card-selected': selectedModelUids.has(model._uid) }">
                      <div class="flex items-center justify-between gap-2">
                        <label class="flex min-w-0 items-center gap-2">
                          <input
                            type="checkbox"
                            class="shrink-0"
                            :checked="selectedModelUids.has(model._uid)"
                            :aria-label="`选择模型 ${model.name || index + 1}`"
                            @change="toggleModelSelected(model._uid)"
                          />
                          <span class="font-medium">模型 {{ index + 1 }}</span>
                        </label>
                        <button type="button" class="channel-switch" :class="{ 'channel-switch-on': model.enabled }" :aria-pressed="model.enabled" @click="model.enabled = !model.enabled"><span></span></button>
                      </div>
                      <div class="mt-3 grid gap-2">
                        <input v-model="model.name" class="input input-mono" placeholder="模型名称" />
                        <select v-model="model.protocol" class="input"><option value="">继承规则</option><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select>
                        <input v-model="model.upstream" class="input input-mono" placeholder="上游映射，留空则同模型名称" />
                        <div class="grid grid-cols-2 gap-2"><input v-model.number="model.input" type="number" step="0.01" min="0" class="input" placeholder="输入价" /><input v-model.number="model.output" type="number" step="0.01" min="0" class="input" placeholder="输出价" /></div>
                        <div class="grid grid-cols-2 gap-2"><button type="button" class="btn btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim() || !headerValidation.valid" @click="testModel(model)">{{ testing[model.name] ? '测试中' : '单测' }}</button><button type="button" class="btn btn-danger btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting" @click="removeModel(index, model)">删除</button></div>
                      </div>
                    </article>
                  </div>
                  <div v-if="!models.length" class="mt-3 rounded-lg border border-dashed border-line p-8 text-center text-sm text-soft">尚未添加模型</div>
                  <div v-if="testRecordRows.length" class="model-table-wrap mt-4">
                    <table class="table-eng min-w-[680px]" aria-label="模型测试记录表">
                      <thead><tr><th>模型</th><th class="w-24">结果</th><th class="w-28">协议</th><th>上游模型</th><th class="w-24 text-right">延迟</th><th>说明</th></tr></thead>
                      <tbody><tr v-for="row in testRecordRows" :key="row.model.name">
                        <td><code class="text-[12px]">{{ row.model.name }}</code></td><td><span v-if="testing[row.model.name] || row.result?.pending" class="chip chip-test">测试中</span><span v-else-if="row.result?.success" class="chip chip-run">通过</span><span v-else class="chip chip-trip">失败</span></td>
                        <td><code class="text-[12px]">{{ row.result?.protocol || '—' }}</code></td><td><code class="break-all text-[12px]">{{ row.result?.upstream || row.model.upstream || row.model.name }}</code></td><td class="num">{{ row.result?.latency_ms ? `${row.result.latency_ms} ms` : '—' }}</td><td class="max-w-md break-words text-[12px] text-soft">{{ row.result?.success ? (row.result.reply || '连通正常') : (row.result?.error || '等待试验结果') }}</td>
                      </tr></tbody>
                    </table>
                  </div>
                </div>
              </ConsoleSection>
            </div>

            <div v-show="editorTab === 'overrides'" class="space-y-3">
              <ConsoleSection title="协议路由规则" description="按模型名称匹配目标协议；优先级低于模型显式协议配置。" eyebrow="Protocol rules">
                <div class="space-y-2">
                  <div v-for="(rule, index) in rules" :key="index" class="rule-row">
                    <input v-model="rule.pattern" class="input input-mono text-[12px]" placeholder="^claude" :aria-label="`第 ${index + 1} 条协议规则正则`" />
                    <select v-model="rule.protocol" class="input text-[12px]"><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select>
                    <button type="button" class="btn btn-danger btn-sm" @click="rules.splice(index, 1)"><ConsoleIcon name="trash" class="h-4 w-4" />删除</button>
                  </div>
                  <button type="button" class="btn btn-sm" @click="rules.push({ pattern: '', protocol: 'anthropic' })"><ConsoleIcon name="plus" class="h-4 w-4" />添加规则</button>
                </div>
              </ConsoleSection>
              <ConsoleSection title="请求内容改写" description="在协议转换后、发送到上游前应用。" eyebrow="Overrides">
                <div class="grid min-w-0 gap-6 xl:grid-cols-2">
                  <HeaderOverrideEditor v-model="form.header_override" :disabled="editorBusy" @validation="headerValidation = $event" />
                  <BodyOverrideEditor v-model="form.body_override" :disabled="editorBusy" @validation="bodyValidation = $event" />
                </div>
              </ConsoleSection>
            </div>

            <div v-show="editorTab === 'reliability'" class="space-y-3">
              <ConsoleSection title="路由参与与负载" description="控制渠道是否参与路由，以及同优先级下的负载权重。" eyebrow="Reliability">
                <div class="grid gap-4 md:grid-cols-2">
                  <div class="rounded-lg border border-line bg-surface p-3"><div class="flex items-center justify-between gap-3"><div><div class="text-sm font-semibold text-ink">渠道状态</div><p class="mt-1 text-xs text-soft">{{ form.status === 1 ? '当前参与模型路由。' : '当前不会接收新请求。' }}</p></div><button type="button" class="channel-switch" :class="{ 'channel-switch-on': form.status === 1 }" :aria-pressed="form.status === 1" @click="form.status = form.status === 1 ? 0 : 1"><span></span></button></div></div>
                  <div><label class="field-label" for="channel-weight">渠道权重</label><input id="channel-weight" v-model.number="form.weight" type="number" min="1" class="input input-mono" placeholder="1" /><p class="mt-1 text-[11px] text-soft">优先级仍由左侧队列拖拽顺序决定。</p></div>
                  <div class="md:col-span-2"><label class="field-label" for="channel-test-prompt">测试提示词覆盖</label><textarea id="channel-test-prompt" v-model="form.test_prompt" class="input min-h-24 resize-y" maxlength="4000" :placeholder="`留空继承全局：${globalTestPrompt}`"></textarea><p class="mt-1 text-[11px] text-soft">单测与批量体检优先使用此内容；留空时继承全局默认。</p></div>
                </div>
              </ConsoleSection>
              <ConsoleSection title="健康检查与熔断" description="对已保存渠道运行全模型检查，或清除累计健康状态。" eyebrow="Health">
                <template #actions><span v-if="selectedChannel" class="chip" :class="healthClass(channelHealth(selectedChannel))">{{ healthText(channelHealth(selectedChannel)) }}</span></template>
                <InlineNotice v-if="!form.id" tone="info" title="保存后可用">创建渠道后即可运行全模型检查和重置健康状态。</InlineNotice>
                <div v-else class="grid gap-3 sm:grid-cols-2">
                  <button type="button" class="btn justify-start" :disabled="checkupLoadingId !== null" @click="checkupChannel(selectedChannel || form)"><ConsoleIcon name="bolt" class="h-4 w-4" />{{ checkupLoadingId === form.id ? '检查中' : '运行全模型检查' }}</button>
                  <button type="button" class="btn justify-start" :disabled="resettingIds.has(form.id)" @click="resetBreaker(selectedChannel || form)"><ConsoleIcon name="arrowPath" class="h-4 w-4" />{{ resettingIds.has(form.id) ? '重置中' : breakerState(selectedChannel || form) === 'trip' ? '解除熔断' : '重置健康状态' }}</button>
                </div>
              </ConsoleSection>
            </div>
          </main>
        </div>
      </div>
      <template #footer>
        <div class="channel-actionbar">
          <div class="min-w-0"><div class="flex items-center gap-2 text-xs font-semibold" :class="`save-state-${saveStatus.tone}`"><span class="save-state-dot"></span>{{ saveStatus.label }}</div><p class="mt-1 truncate text-[10px] text-soft">{{ saveHint }}</p></div>
          <div class="channel-action-buttons">
            <button v-if="form.id" type="button" class="btn btn-danger" :disabled="editorBusy || deletingIds.has(form.id)" @click="removeChannel(selectedChannel || form)"><ConsoleIcon name="trash" class="h-4 w-4" />{{ deletingIds.has(form.id) ? '删除中' : '删除' }}</button>
            <button type="button" class="btn" :disabled="editorBusy" @click="closeEditor">取消</button>
            <button type="button" class="btn btn-primary" :disabled="editorBusy || !canSave || (!isDirty && Boolean(form.id))" @click="save"><ConsoleIcon name="checkCircle" class="h-4 w-4" />{{ saving ? '保存中' : form.id ? '保存更改' : '创建渠道' }}</button>
          </div>
        </div>
      </template>
    </Drawer>

    <Modal :open="showCheckup" :title="`渠道检查记录 · ${checkupChannelName}`" width="max-w-4xl" @close="showCheckup = false">
      <div class="space-y-3">
        <div v-if="checkupSummary" class="flex flex-wrap items-center gap-2"><span class="chip chip-run">通过 {{ checkupSummary.success }}</span><span class="chip chip-trip">失败 {{ checkupSummary.failed }}</span><span class="chip chip-test">总计 {{ checkupSummary.total }}</span><span class="chip">合格率 {{ checkupRate }}%</span></div>
        <div v-if="checkupResults.length" class="space-y-2">
          <article v-for="(result, index) in checkupResults" :key="`${result.model}-${index}`" class="rounded-lg border border-line p-3">
            <div class="flex flex-wrap items-center justify-between gap-2"><code class="break-all text-[12px]">{{ result.model }}</code><span class="chip" :class="result.success ? 'chip-run' : 'chip-trip'">{{ result.success ? '通过' : '失败' }}</span></div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-soft"><span>协议：<code>{{ result.protocol || '—' }}</code></span><span>上游：<code>{{ result.upstream || result.model }}</code></span><span>延迟：<code>{{ result.success && result.latency_ms ? `${result.latency_ms} ms` : '—' }}</code></span></div>
            <p class="mt-2 break-words text-xs leading-5 text-soft">{{ result.success ? (result.reply || '连通正常') : (result.error || '未返回错误说明') }}</p>
          </article>
        </div>
        <div v-else class="rounded-lg border border-dashed border-line px-4 py-8 text-center text-sm text-soft">无检查记录</div>
      </div>
      <template #footer><button type="button" class="btn" @click="showCheckup = false">关闭记录</button></template>
    </Modal>
  </div>
</template>

<style scoped>
.channel-console { display: flex; min-width: 0; max-height: calc(100dvh - 184px); min-height: 26rem; flex-direction: column; }
.channel-list-scroll { min-height: 0; flex: 1; overflow-y: auto; overscroll-behavior: contain; }
.channel-table { min-width: 60rem; table-layout: fixed; }
.channel-table tbody tr { transition: background-color 150ms ease, opacity 150ms ease; }
.channel-table tbody tr:focus-visible { outline: 2px solid rgb(var(--color-accent)); outline-offset: -2px; }
.channel-row-selected td { background: rgb(var(--color-accent-muted)); }
.channel-row-selected td:first-child { box-shadow: inset 3px 0 0 rgb(var(--color-accent)); }
.channel-row-off { opacity: .66; }
.channel-row-dragging { opacity: .4; cursor: grabbing; }
.channel-row-dropzone td { box-shadow: inset 0 2px 0 #a4382f; }
.channel-card-list > * + * { border-top: 1px solid rgb(var(--color-border)); }
.channel-card { transition: background-color 150ms ease; }
.channel-card:hover, .channel-card:focus-visible { background: rgb(var(--color-surface-1)); outline: none; }
.channel-card-selected { border-left-color: rgb(var(--color-accent)); background: rgb(var(--color-accent-muted)); }
.channel-grip { display: inline-flex; height: 26px; width: 24px; align-items: center; justify-content: center; border-radius: 5px; color: rgb(var(--color-text-muted)); cursor: grab; }
.channel-grip:hover:not(:disabled) { background: rgb(var(--color-surface-2)); color: rgb(var(--color-text)); }
.channel-grip:disabled { cursor: not-allowed; opacity: .35; }
.channel-state-dot { position: relative; display: inline-flex; width: 9px; height: 9px; flex: 0 0 auto; border-radius: 999px; background: #938a7c; }
.channel-state-dot i { position: absolute; inset: 3px; border-radius: inherit; background: white; opacity: .7; }
.channel-state-run { background: #50705a; box-shadow: 0 0 0 3px rgba(80,112,90,.12); }
.channel-state-test { background: #9a6a2f; box-shadow: 0 0 0 3px rgba(154,106,47,.12); }
.channel-state-trip { background: #a4382f; box-shadow: 0 0 0 3px rgba(164,56,47,.12); }
.channel-state-off { background: #938a7c; }
.channel-detail { display: flex; min-width: 0; flex-direction: column; gap: 12px; }
.channel-detail-meta { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid rgb(var(--color-border)); padding-bottom: 10px; }
.channel-detail-layout { display: grid; min-width: 0; align-items: start; gap: 16px; grid-template-columns: 176px minmax(0, 1fr); }
.channel-detail-nav { position: sticky; top: 0; border-right: 1px solid rgb(var(--color-border)); padding: 0 8px 12px 0; }
.channel-detail-nav nav button { display: grid; width: 100%; grid-template-columns: auto minmax(0,1fr) auto; align-items: center; gap: 8px; border-radius: 6px; padding: 9px 8px; text-align: left; color: rgb(var(--color-text-secondary)); transition: background-color 140ms ease, color 140ms ease; }
.channel-detail-nav nav button:hover { background: rgb(var(--color-surface-2)); color: rgb(var(--color-text)); }
.channel-detail-nav nav button[aria-selected='true'] { background: rgb(var(--color-accent-muted)); color: rgb(var(--color-accent-strong)); }
.channel-detail-nav b { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; font-weight: 650; }
.channel-detail-nav small { display: block; overflow: hidden; margin-top: 2px; text-overflow: ellipsis; white-space: nowrap; font-size: 9px; color: rgb(var(--color-text-muted)); }
.detail-nav-state { width: 6px; height: 6px; border-radius: 999px; background: rgb(var(--color-text-muted)); }
.detail-nav-state.is-done { background: #50705a; }
.detail-nav-state.is-pending { background: #9a6a2f; }
.detail-summary { display: grid; gap: 6px; margin: 18px 0 0; border-top: 1px solid rgb(var(--color-border)); padding-top: 12px; }
.detail-summary div { display: flex; min-width: 0; justify-content: space-between; gap: 8px; font-size: 9px; color: rgb(var(--color-text-muted)); }
.detail-summary dd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: 'Spline Sans Mono', monospace; color: rgb(var(--color-text-secondary)); }
.channel-detail-content { min-width: 0; }
.detail-mobile-nav { display: none; }
.channel-key-row { display: flex; min-width: 0; gap: 8px; }
.model-table-wrap { max-width: 100%; overflow-x: auto; border: 1px solid rgb(var(--color-border)); }
/* 选中的模型行/卡片沿用渠道队列的选中视觉，保持两处交互观感一致 */
.model-row-selected td { background: rgb(var(--color-accent-muted)); }
.model-row-selected td:first-child { box-shadow: inset 3px 0 0 rgb(var(--color-accent)); }
.model-card-selected { border-color: rgb(var(--color-accent) / .45); background: rgb(var(--color-accent-muted)); }

.model-bulk-bar {
  display: grid;
  gap: 10px;
  border: 1px solid rgb(var(--color-accent) / .35);
  border-radius: 8px;
  background: rgb(var(--color-accent-muted));
  padding: 10px 12px;
}
.model-bulk-head { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 8px; }
.model-bulk-actions { display: flex; min-width: 0; flex-wrap: wrap; gap: 6px; }
.model-bulk-fields { display: grid; min-width: 0; gap: 10px; grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr); }
.model-bulk-field { display: grid; min-width: 0; gap: 4px; }
@media (max-width: 900px) {
  .model-bulk-fields { grid-template-columns: minmax(0, 1fr); }
}
.rule-row { display: grid; min-width: 0; grid-template-columns: minmax(0,1fr) 140px auto; gap: 8px; }
.channel-actionbar { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
.channel-action-buttons { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
.save-state-dot { width: 7px; height: 7px; border-radius: 999px; background: currentColor; }
.save-state-saving { color: rgb(var(--color-warning)); }.save-state-dirty { color: rgb(var(--color-danger)); }.save-state-saved { color: rgb(var(--color-success)); }.save-state-idle { color: rgb(var(--color-text-secondary)); }
@media (max-width: 1100px) {
  .channel-detail-layout { grid-template-columns: 152px minmax(0,1fr); gap: 12px; }
  .channel-key-row { flex-wrap: wrap; }.channel-key-row .input { flex-basis: 100%; }
}
@media (max-width: 767px) {
  .channel-console { max-height: none; min-height: 0; }.channel-list-scroll { overflow: visible; }
  .detail-mobile-nav { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); gap: 4px; border-bottom: 1px solid rgb(var(--color-border)); padding-bottom: 8px; }
  .detail-mobile-nav button { display: flex; min-width: 0; flex-direction: column; align-items: center; justify-content: center; gap: 4px; border-radius: 6px; padding: 7px 3px; color: rgb(var(--color-text-secondary)); font-size: 9px; font-weight: 600; }
  .detail-mobile-nav button[aria-selected='true'] { background: rgb(var(--color-accent-muted)); color: rgb(var(--color-accent-strong)); }
  .channel-detail-layout { display: block; }.channel-detail-nav { display: none; }
  .channel-action-buttons { width: 100%; }.channel-action-buttons .btn { min-width: 0; flex: 1; padding-inline: 8px; }
}
@media (max-width: 520px) {
  .channel-key-row { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); }.channel-key-row .input { grid-column: 1 / -1; }
  .rule-row { grid-template-columns: minmax(0,1fr); }.rule-row .btn { width: 100%; }.channel-detail-meta .chip { display: none; }
}
@media (max-width: 390px) { .detail-mobile-nav button { font-size: 8px; }.channel-actionbar { gap: 8px; }.channel-action-buttons { gap: 5px; } }
@media (prefers-reduced-motion: reduce) { .channel-table tbody tr, .channel-card { transition: none; } }
</style>
