<script setup lang='ts'>
import { ref, computed, onMounted, nextTick } from 'vue'
import { getNodes, getNodeOverview, AddNodes, DelNode, UpdateNode, GetGroupFull, SetGroup, UnbindGroup, DelGroup, SetNodeVisibility, SetGroupVisibility } from "@/api/subcription/node"
import { countryFlag } from "@/utils/flag"
import NodeUnlockDialog from "@/components/NodeUnlockDialog.vue"
import NodeTcpDialog from "@/components/NodeTcpDialog.vue"
import NodeQualityDialog from "@/components/NodeQualityDialog.vue"

interface OverviewItem {
  id: number
  name: string
  link: string
  server: string
  country: string
  countryCode: string
  rtt: number
  groups: string[]
  score: number
  availability: number
  averageRtt: number
  jitter: number
  sampleCount: number
  lastTestedAt: string
}
interface NodeItem {
  ID: number
  Name: string
  Link: string
  Hidden?: boolean
  GroupNodes?: { ID: number; Name: string; Hidden?: boolean }[]
}
interface GroupDetail { id:number; name:string; node_count:number; hidden:boolean }

// ===== 状态 =====
const loading = ref(false)
const viewMode = ref<'card' | 'list'>('card')   // 卡片/列表
const overviewList = ref<OverviewItem[]>([])     // 概览（含国家/延迟）
const fullNodes = ref<NodeItem[]>([])            // 完整节点（含 Link 供编辑/复制）
const allGroupNames = ref<string[]>([])          // 分组名列表
const groupDetails = ref<GroupDetail[]>([])      // 含隐藏状态的完整分组列表
const manageGroupId = ref<number | null>(null)
const activeGroup = ref('全部')                  // 当前选中分组
const searchText = ref('')
const filterCountries = ref<string[]>([])        // 国家筛选（多选）

// 表格选择
const multipleSelection = ref<any[]>([])
const multipleTable = ref<any>(null)

// 卡片视图勾选（跨国家分组全局）
const selectedIds = ref<Set<number>>(new Set())
const selectedNodes = computed(() => filteredNodes.value.filter(n => selectedIds.value.has(n.id)))
const selectedCount = computed(() => selectedIds.value.size)

// 卡片勾选切换
const toggleCardSelect = (id: number) => {
  const s = new Set(selectedIds.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selectedIds.value = s
}
// 按国家全选/取消
const toggleCountrySelect = (items: OverviewItem[]) => {
  const ids = items.map(i => i.id)
  const allSelected = ids.every(i => selectedIds.value.has(i))
  const s = new Set(selectedIds.value)
  for (const i of ids) {
    if (allSelected) s.delete(i)
    else s.add(i)
  }
  selectedIds.value = s
}
// 国家是否全选（用于按钮态）
const countryAllSelected = (items: OverviewItem[]) => items.length > 0 && items.every((i: OverviewItem) => selectedIds.value.has(i.id))
// 全局全选/取消
const toggleAllNodes = () => {
  const allIds = filteredNodes.value.map(n => n.id)
  const allSelected = allIds.length > 0 && allIds.every(i => selectedIds.value.has(i))
  const s = new Set(selectedIds.value)
  for (const i of allIds) {
    if (allSelected) s.delete(i)
    else s.add(i)
  }
  selectedIds.value = s
}
// 清空选择
const clearCardSelect = () => { selectedIds.value = new Set() }

// 批量删除选中（卡片视图）
const cardSelectDel = async () => {
  const ids = [...selectedIds.value]
  if (!ids.length) { ElMessage.warning('请选择要删除的节点'); return }
  try {
    await ElMessageBox.confirm(`是否删除选中的 ${ids.length} 条节点 ?`, '提示', { type: 'warning' })
    for (const id of ids) await DelNode({ id })
    ElMessage.success('批量删除成功')
  } catch { /* cancel */ }
  clearCardSelect(); loadAll()
}
// 批量移出当前分组（仅 activeGroup≠全部 时）
const cardSelectUnbind = async () => {
  const ids = [...selectedIds.value]
  if (!ids.length) { ElMessage.warning('请选择节点'); return }
  try {
    await ElMessageBox.confirm(`是否将选中的 ${ids.length} 条节点移出分组「${activeGroup.value}」？仅解除绑定，不会删除节点。`, '提示', { type: 'warning' })
    for (const id of ids) {
      const n = filteredNodes.value.find(x => x.id === id)
      if (n) await UnbindGroup({ name: n.name, group: activeGroup.value })
    }
    ElMessage.success('已移出分组')
  } catch { /* cancel */ }
  clearCardSelect(); loadAll()
}

// 弹窗
const Nodedialog = ref(false)
const Groupdialog = ref(false)
const dialogMode = ref<'add' | 'edit'>('add')
const RadioGroup = ref('1')
const NodeForm = ref({ ID: 0, Title: '', Name: '', Link: '', GroupName: [] as string[] })
const SelectionNodeGroups = ref<string[]>([])
const NodeGroupInput = ref('')
const SelectionNode = ref('')
const delGroupName = ref('')
const hiddenNodeCount = computed(() => fullNodes.value.filter(n => nodeEffectiveHidden(n)).length)
const manageGroupNodes = computed(() => {
  const group = groupDetails.value.find(g => g.id === manageGroupId.value)
  if (!group) return []
  return fullNodes.value.filter(n => (n.GroupNodes || []).some(g => g.ID === group.id))
})
const nodeEffectiveHidden = (node:any) => !!node?.Hidden || (node?.GroupNodes || []).some((g:any) => !!g.Hidden)

// ===== 派生数据 =====
// 可用国家列表（用于筛选）
const countryOptions = computed(() => {
  const s = new Set<string>()
  overviewList.value.forEach(n => { if (n.country) s.add(n.country) })
  return [...s].sort()
})

// 按分组+搜索+国家 过滤
const filteredNodes = computed(() => {
  return overviewList.value.filter(n => {
    if (activeGroup.value !== '全部' && !(n.groups || []).includes(activeGroup.value)) return false
    if (searchText.value && !n.name.toLowerCase().includes(searchText.value.toLowerCase())) return false
    if (filterCountries.value.length && !filterCountries.value.includes(n.country)) return false
    return true
  })
})

// 分组树数据
const groupTree = computed(() => {
  const nodes: any[] = []
  for (const g of allGroupNames.value) {
    const count = overviewList.value.filter(n => (n.groups || []).includes(g)).length
    nodes.push({ id: g, label: g, count })
  }
  return nodes
})
const groupedVisibleCount = computed(() => overviewList.value.filter(n => (n.groups || []).length > 0).length)

// 延迟颜色
const rttColor = (rtt: number) => {
  if (rtt === -2) return "#95a5a6"
  if (rtt < 0) return "#95a5a6"
  if (rtt < 100) return "#2ecc71"
  if (rtt < 300) return "#f1c40f"
  return "#e74c3c"
}
const rttText = (rtt: number) => rtt === -2 ? "测试中…" : (rtt < 0 ? "超时" : rtt + "ms")

// ===== 测试弹窗状态 =====
const unlockDialogVisible = ref(false)
const tcpDialogVisible = ref(false)
const testNode = ref<{ id: number; name: string } | null>(null)
const qualityNode = ref<OverviewItem | null>(null)
const qualityDialogVisible = ref(false)
const openUnlock = (node: OverviewItem) => { testNode.value = { id: node.id, name: node.name }; unlockDialogVisible.value = true }
const openTcp = (node: OverviewItem) => { testNode.value = { id: node.id, name: node.name }; tcpDialogVisible.value = true }
const openQuality = (node: OverviewItem) => { qualityNode.value = node; qualityDialogVisible.value = true }

import { testLocalAll } from "@/utils/ping"

// ===== 数据加载 =====
const localTesting = ref(false)

const loadAll = async () => {
  loading.value = true
  try {
    const [ov, nd, gp] = await Promise.all([
      getNodeOverview(), getNodes(true), GetGroupFull(true)
    ])
    const data = ov?.data || []
    data.forEach((n: any) => {
      n.rtt = n.rtt === -1 ? -1 : -2; // VPS marks dead as -1, otherwise set to testing state
    })
    overviewList.value = data
    fullNodes.value = nd?.data || []
    groupDetails.value = Array.isArray(gp?.data) ? gp.data : []
    allGroupNames.value = groupDetails.value.filter(g => !g.hidden).map(g => g.name)
    if (!manageGroupId.value && groupDetails.value.length) manageGroupId.value = groupDetails.value[0].id
    
    triggerLocalPing()
  } catch { /* ignore */ } finally {
    loading.value = false
  }
}

const openGroupManager = () => {
  if (!manageGroupId.value && groupDetails.value.length) manageGroupId.value = groupDetails.value[0].id
  Groupdialog.value = true
}
const toggleGroupHidden = async (group:any) => {
  const next = !group.hidden
  await SetGroupVisibility(group.id, next)
  ElMessage.success(next ? `分组「${group.name}」已全局隐藏` : `分组「${group.name}」已恢复显示`)
  if (next && activeGroup.value === group.name) activeGroup.value = '全部'
  await loadAll()
}
const toggleNodeHidden = async (node:any) => {
  const next = !node.Hidden
  await SetNodeVisibility(node.ID, next)
  ElMessage.success(next ? `节点「${node.Name}」已全局隐藏` : `节点「${node.Name}」已恢复显示`)
  await loadAll()
}
const hideVisibleNode = async (id:number) => {
  const node = fullNodes.value.find(n => n.ID === id)
  if (node) await toggleNodeHidden(node)
}
const hideActiveGroup = async () => {
  const group = groupDetails.value.find(g => g.name === activeGroup.value)
  if (!group) return
  await ElMessageBox.confirm(`隐藏分组「${group.name}」后，该组全部节点会在 SubLinkX 全局隐藏，但不会删除数据。继续？`, '全局隐藏分组', { type:'warning' })
  await toggleGroupHidden(group)
}
const hideSelectedNodes = async () => {
  const ids = [...selectedIds.value]
  if (!ids.length) return
  await ElMessageBox.confirm(`将选中的 ${ids.length} 个节点全局隐藏？节点数据不会删除。`, '隐藏节点', { type:'warning' })
  for (const id of ids) await SetNodeVisibility(id, true)
  clearCardSelect(); await loadAll(); ElMessage.success('选中节点已全局隐藏')
}
const hideTableSelectedNodes = async () => {
  const ids = multipleSelection.value.map((item:any) => Number(item.id)).filter((id:number) => id > 0)
  if (!ids.length) { ElMessage.warning('请选择要隐藏的节点'); return }
  await ElMessageBox.confirm(`将选中的 ${ids.length} 个节点全局隐藏？节点数据不会删除，可在「分组 → 管理」中恢复。`, '隐藏节点', { type:'warning' })
  for (const id of ids) await SetNodeVisibility(id, true)
  await loadAll(); ElMessage.success('选中节点已全局隐藏')
}
const hideGroupByName = async (name:string) => {
  const group = groupDetails.value.find(g => g.name === name)
  if (!group) return
  await ElMessageBox.confirm(`隐藏分组「${group.name}」后，该组节点会在 SubLinkX 全局隐藏，但数据仍保留。继续？`, '隐藏整个分组', { type:'warning' })
  await toggleGroupHidden(group)
}

const triggerLocalPing = async () => {
  localTesting.value = true
  try {
    await testLocalAll(overviewList.value, (index, rtt) => {
      if (overviewList.value[index]) {
        overviewList.value[index].rtt = rtt
      }
    })
  } finally {
    localTesting.value = false
  }
}

// ===== 添加/编辑节点 =====
const handleAddNode = () => {
  dialogMode.value = 'add'
  Nodedialog.value = true
  NodeForm.value = { ID: 0, Title: '添加节点', Name: '', Link: '', GroupName: [] }
  RadioGroup.value = allGroupNames.value.length ? '1' : '2'
  SelectionNodeGroups.value = []
  NodeGroupInput.value = ''
}
const handleEditNode = (row: any) => {
  const full = fullNodes.value.find(n => n.ID === row.id)
  dialogMode.value = 'edit'
  Nodedialog.value = true
  NodeForm.value = {
    ID: row.id, Title: '编辑节点', Name: row.name, Link: full?.Link || row.link,
    GroupName: (full?.GroupNodes || []).map(g => g.Name),
  }
  SelectionNodeGroups.value = NodeForm.value.GroupName || []
  SelectionNode.value = row.name
}
const SubmitNodeForm = async () => {
  const isAdd = dialogMode.value === 'add'
  const chosenGroups = RadioGroup.value === '1' ? SelectionNodeGroups.value : NodeGroupInput.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!chosenGroups.length) { ElMessage.warning('节点必须至少选择一个分组'); return }
  const links = NodeForm.value.Link.trim().split(/[\n,]/).map(i => i.trim()).filter(i => i)
  if (isAdd && links.length === 0) { ElMessage.warning('节点链接不能为空'); return }
  try {
    if (isAdd) {
      for (const link of links) {
        await AddNodes({ link, group: chosenGroups.join(',') })
      }
      ElMessage.success('节点添加成功')
    } else {
      await UpdateNode({
        id: NodeForm.value.ID, name: NodeForm.value.Name, link: NodeForm.value.Link,
        group: chosenGroups.join(','),
      })
      ElMessage.success('节点更新成功')
    }
  } catch { ElMessage.error(isAdd ? '添加失败' : '更新失败') }
  loadAll(); ClearInput()
}

// ===== 删除/复制 =====
const copyUrl = (url: string) => {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(url).then(() => ElMessage.success('链接已复制')).catch(() => ElMessage.error('复制失败'))
  } else {
    const ta = document.createElement('textarea'); ta.value = url; document.body.appendChild(ta); ta.select()
    try { document.execCommand('copy'); ElMessage.success('已复制') } catch { ElMessage.warning('复制失败') }
    document.body.removeChild(ta)
  }
}
const copyInfo = (row: any) => {
  const full = fullNodes.value.find(n => n.ID === row.id)
  copyUrl(full?.Link || row.link)
}
const handleDel = async (row: any) => {
  try {
    await ElMessageBox.confirm(`你是否要删除 ${row.name} ?`, '提示', { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' })
    await DelNode({ id: row.id })
    ElMessage.success('删除成功')
  } catch { /* cancel */ }
  loadAll(); ClearInput()
}
const selectDel = async () => {
  if (!multipleSelection.value.length) { ElMessage.warning('请选择要删除的节点'); return }
  try {
    await ElMessageBox.confirm(`是否删除选中的 ${multipleSelection.value.length} 条节点 ?`, '提示', { type: 'warning' })
    for (const item of multipleSelection.value) await DelNode({ id: item.id })
    ElMessage.success('批量删除成功')
  } catch { /* cancel */ }
  loadAll()
}
const selectCopy = async () => {
  if (!multipleSelection.value.length) { ElMessage.warning('请选择节点'); return }
  const links = multipleSelection.value.map(i => i.link || i.Link).join('\n')
  copyUrl(links)
}
const selectAll = () => nextTick(() => multipleTable.value?.toggleAllSelection())
const selectClear = () => nextTick(() => multipleTable.value?.clearSelection())

// ===== 分组绑定 =====
const AddGroup = async () => {
  if (RadioGroup.value === '1' && !SelectionNodeGroups.value.length) { ElMessage.warning('请选择分组'); return }
  if (RadioGroup.value === '2' && !NodeGroupInput.value.trim()) { ElMessage.warning('分组名不能为空'); return }
  if (!SelectionNode.value) { ElMessage.warning('请选择节点'); return }
  try {
    await SetGroup({ name: SelectionNode.value, group: RadioGroup.value === '1' ? SelectionNodeGroups.value.join(',') : NodeGroupInput.value })
    ElMessage.success('分组绑定成功')
  } catch { ElMessage.error('绑定失败') }
  loadAll(); ClearInput()
}

// ===== 分组删除 =====
const delGroup = async (group: string) => {
  if (!group) { ElMessage.warning('请选择分组'); return }
  try {
    await ElMessageBox.confirm(`是否删除分组「${group}」？该分组下节点不会被删除，仅解除绑定。`, '提示', { type: 'warning' })
    await DelGroup({ name: group })
    ElMessage.success('分组已删除')
    if (activeGroup.value === group) activeGroup.value = '全部'
    loadAll()
  } catch { /* cancel */ }
}

const ClearInput = () => {
  SelectionNode.value = ''
  NodeForm.value = { ID: 0, Title: '', Name: '', Link: '', GroupName: [] }
  NodeGroupInput.value = ''
  SelectionNodeGroups.value = []
  Nodedialog.value = false
  Groupdialog.value = false
}

// 卡片视图分组（按国家分组展示）
const cardGroups = computed(() => {
  const byCountry: Record<string, OverviewItem[]> = {}
  for (const n of filteredNodes.value) {
    const key = n.country || '未知'
    if (!byCountry[key]) byCountry[key] = []
    byCountry[key].push(n)
  }
  return Object.keys(byCountry).sort().map(c => ({ country: c, items: byCountry[c] }))
})

const handleSelectionChange = (val: any[]) => { multipleSelection.value = val }

onMounted(loadAll)

// ===== 控制台统计与批量命令 =====
const nodeStats = computed(() => {
  const list = overviewList.value
  const reachable = list.filter(n => n.rtt >= 0).length
  const timeout = list.filter(n => n.rtt < 0 && n.rtt !== -2).length
  const rtts = list.filter(n => n.rtt >= 0).map(n => n.rtt)
  const avgRtt = rtts.length ? Math.round(rtts.reduce((a, b) => a + b, 0) / rtts.length) : 0
  const scores = list.map(n => n.score || 0).filter(s => s > 0)
  const avgScore = scores.length ? Math.round(scores.reduce((a, b) => a + b, 0) / scores.length) : 0
  return [
    { label: '节点总数', value: list.length },
    { label: '链路可达', value: reachable, tone: 'ok' as const },
    { label: '超时', value: timeout, tone: timeout > 0 ? ('bad' as const) : ('default' as const) },
    { label: '平均延迟', value: avgRtt, unit: 'ms' },
    { label: '平均评分', value: avgScore, tone: 'warn' as const },
  ]
})

const onBatchCommand = (cmd: string) => {
  switch (cmd) {
    case 'toggleAll': toggleAllNodes(); break
    case 'clear': clearCardSelect(); break
    case 'unbind': cardSelectUnbind(); break
    case 'hide': hideSelectedNodes(); break
    case 'hideGroup': hideActiveGroup(); break
    case 'delete': cardSelectDel(); break
  }
}

const onNodeCommand = (cmd: string, n: any) => {
  switch (cmd) {
    case 'copy': copyInfo(n); break
    case 'hide': hideVisibleNode(n.id); break
    case 'delete': handleDel(n); break
  }
}
</script>

<template>
  <div class="p-4 md:p-6 lg:p-8 h-full flex flex-col gap-6">
    <PageHero title="节点列表" desc="节点、分组与质量一览。卡片视图关注健康度，列表视图用于批量维护。">
      <template #actions>
        <el-button :loading="localTesting" @click="triggerLocalPing">测本地</el-button>
        <el-button :loading="loading" @click="loadAll">刷新</el-button>
        <el-button type="primary" @click="handleAddNode">添加节点</el-button>
      </template>
      <template #stats>
        <StatStrip :items="nodeStats" />
      </template>
    </PageHero>

    <div class="nodes-layout">
      <aside class="group-rail">
        <header class="rail-head">
          <span>分组</span>
          <button class="rail-manage" @click="openGroupManager">管理</button>
        </header>
        <button class="rail-item" :class="{ active: activeGroup === '全部' }" @click="activeGroup = '全部'">
          <span class="rail-name">全部节点</span>
          <b class="rail-count">{{ loading ? '…' : overviewList.length }}</b>
        </button>
        <button v-for="g in groupTree" :key="g.id" class="rail-item" :class="{ active: activeGroup === g.id }" @click="activeGroup = g.id">
          <span class="rail-name">{{ g.label }}</span>
          <span class="rail-meta">
            <b class="rail-count">{{ g.count }}</b>
            <i class="rail-hide" title="隐藏整组" @click.stop="hideGroupByName(g.id)">隐藏</i>
          </span>
        </button>
        <p class="rail-foot">已分组 {{ groupedVisibleCount }}/{{ overviewList.length }}<template v-if="hiddenNodeCount"> · 隐藏 {{ hiddenNodeCount }}</template></p>
      </aside>

      <section class="nodes-main">
        <div class="nodes-toolbar">
          <el-input v-model="searchText" placeholder="搜索节点名" clearable class="toolbar-search" />
          <el-select v-model="filterCountries" multiple collapse-tags placeholder="国家筛选" class="toolbar-country">
            <el-option v-for="c in countryOptions" :key="c" :label="c" :value="c" />
          </el-select>
          <div class="view-switch">
            <button :class="{ active: viewMode === 'card' }" @click="viewMode = 'card'">卡片</button>
            <button :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'">列表</button>
          </div>
          <div class="toolbar-spacer"></div>
          <el-dropdown v-if="viewMode === 'card'" trigger="click" @command="onBatchCommand">
            <el-button :disabled="!selectedCount">
              批量操作<span v-if="selectedCount">（{{ selectedCount }}）</span>
              <span class="batch-caret" aria-hidden="true"></span>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="toggleAll">{{ filteredNodes.length && filteredNodes.every(n => selectedIds.has(n.id)) ? '取消全选' : '全选当前' }}</el-dropdown-item>
                <el-dropdown-item command="clear" :disabled="!selectedCount">取消选择</el-dropdown-item>
                <el-dropdown-item v-if="activeGroup !== '全部'" command="unbind" :disabled="!selectedCount">移出分组</el-dropdown-item>
                <el-dropdown-item command="hide" :disabled="!selectedCount">隐藏选中</el-dropdown-item>
                <el-dropdown-item v-if="activeGroup !== '全部'" command="hideGroup">隐藏本组</el-dropdown-item>
                <el-dropdown-item command="delete" divided :disabled="!selectedCount">删除选中</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>

        <div class="nodes-body" v-loading="loading">
          <template v-if="viewMode === 'card'">
            <div v-for="g in cardGroups" :key="g.country" class="country-block">
              <header class="country-head">
                <el-checkbox :model-value="countryAllSelected(g.items)" @change="() => toggleCountrySelect(g.items)" size="small" />
                <span class="country-flag">{{ countryFlag(g.items[0].countryCode) }}</span>
                <span class="country-name">{{ g.country }}</span>
                <b class="country-count">{{ g.items.length }}</b>
                <button class="country-select" @click="toggleCountrySelect(g.items)">{{ countryAllSelected(g.items) ? '取消全选' : '全选' }}</button>
              </header>
              <div class="node-grid">
                <article v-for="n in g.items" :key="n.id" class="node-card" :class="{ selected: selectedIds.has(n.id) }">
                  <header class="node-card-top">
                    <el-checkbox :model-value="selectedIds.has(n.id)" @change="() => toggleCardSelect(n.id)" size="small" />
                    <span class="node-flag">{{ countryFlag(n.countryCode) }}</span>
                    <span class="node-name" :title="n.name">{{ n.name }}</span>
                    <QualityRing :value="n.score || 0" :size="42" />
                  </header>
                  <div class="node-meta">{{ n.country }} · {{ n.server }}</div>
                  <div class="node-stats">
                    <span class="node-rtt" :class="n.rtt === -2 ? 'unknown' : n.rtt < 0 ? 'bad' : n.rtt < 100 ? 'good' : n.rtt < 300 ? 'warn' : 'bad'">{{ rttText(n.rtt) }}</span>
                    <span>可用 {{ n.availability ?? 0 }}%</span>
                    <span>抖动 {{ n.jitter ?? 0 }}ms</span>
                  </div>
                  <footer class="node-actions">
                    <button @click="openQuality(n)">质量</button>
                    <button @click="openUnlock(n)">解锁</button>
                    <button @click="openTcp(n)">TCP</button>
                    <button @click="handleEditNode(n)">编辑</button>
                    <el-dropdown trigger="click" @command="cmd => onNodeCommand(cmd, n)">
                      <button class="node-more">更多</button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item command="copy">复制链接</el-dropdown-item>
                          <el-dropdown-item command="hide">隐藏节点</el-dropdown-item>
                          <el-dropdown-item command="delete" divided>删除节点</el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </footer>
                </article>
              </div>
            </div>
            <EmptyState v-if="!loading && filteredNodes.length === 0" title="没有匹配的节点" desc="调整搜索词、国家筛选，或切换到其他分组再试。" />
          </template>

          <template v-else>
            <el-table ref="multipleTable" :data="filteredNodes" stripe style="width: 100%" @selection-change="handleSelectionChange">
              <el-table-column type="selection" width="45" />
              <el-table-column label="节点" min-width="200">
                <template #default="{ row }">
                  <span class="row-flag">{{ countryFlag(row.countryCode) }}</span>
                  <span class="row-name">{{ row.name }}</span>
                </template>
              </el-table-column>
              <el-table-column label="国家" width="90">
                <template #default="{ row }">{{ row.country }}</template>
              </el-table-column>
              <el-table-column label="延迟" width="90">
                <template #default="{ row }">
                  <el-tag :type="row.rtt === -2 ? 'info' : row.rtt < 0 ? 'danger' : row.rtt < 100 ? 'success' : row.rtt < 300 ? 'warning' : 'danger'" size="small">
                    {{ rttText(row.rtt) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="质量" width="90">
                <template #default="{ row }"><el-tag :type="row.score >= 80 ? 'success' : row.score >= 60 ? 'warning' : 'danger'" size="small">{{ row.score || 0 }}分</el-tag></template>
              </el-table-column>
              <el-table-column label="分组" min-width="120">
                <template #default="{ row }">
                  <el-tag v-for="g in (row.groups || [])" :key="g" size="small" effect="plain">{{ g }}</el-tag>
                  <span v-if="!(row.groups || []).length" class="muted">未分组</span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="230">
                <template #default="{ row }">
                  <el-button link type="primary" size="small" @click="handleEditNode(row)">编辑</el-button>
                  <el-button link type="primary" size="small" @click="copyInfo(row)">复制</el-button>
                  <el-button link type="warning" size="small" @click="hideVisibleNode(row.id)">隐藏</el-button>
                  <el-button link type="danger" size="small" @click="handleDel(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="list-actions">
              <el-button size="small" @click="selectAll">全选</el-button>
              <el-button size="small" @click="selectClear">取消</el-button>
              <el-button size="small" type="primary" @click="selectCopy">复制选中</el-button>
              <el-button size="small" type="warning" plain @click="hideTableSelectedNodes" :disabled="!multipleSelection.length">隐藏选中</el-button>
              <el-button size="small" type="danger" @click="selectDel">删除选中</el-button>
            </div>
          </template>
        </div>
      </section>
    </div>

    <!-- 添加/编辑节点弹窗 -->
    <el-dialog v-model="Nodedialog" :title="NodeForm.Title" width="560px" align-center>
      <el-form label-position="top" class="node-form">
        <el-divider content-position="left">基本信息</el-divider>
        <el-form-item label="节点链接">
          <el-input v-model="NodeForm.Link" placeholder="节点链接，支持多行（回车/逗号分隔）" type="textarea" :autosize="{ minRows: 3, maxRows: 8 }" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'edit'" label="节点名称">
          <el-input v-model="NodeForm.Name" placeholder="节点名称（编辑时）" />
        </el-form-item>

        <el-divider content-position="left">分组</el-divider>
        <el-radio-group v-model="RadioGroup" size="small" class="group-tabs">
          <el-radio-button v-if="allGroupNames.length" value="1">选择已有分组</el-radio-button>
          <el-radio-button value="2">创建新分组</el-radio-button>
        </el-radio-group>
        <div v-if="RadioGroup === '1' && allGroupNames.length" class="group-field">
          <el-select v-model="SelectionNodeGroups" multiple placeholder="选择分组（至少一个）" class="full">
            <el-option v-for="g in allGroupNames" :key="g" :label="g" :value="g" />
          </el-select>
        </div>
        <div v-else class="group-field">
          <el-input v-model="NodeGroupInput" placeholder="输入新分组名" />
        </div>
      </el-form>
      <template #footer>
        <el-button text @click="Nodedialog = false">取消</el-button>
        <el-button type="primary" @click="SubmitNodeForm">{{ dialogMode === 'add' ? '添加节点' : '保存修改' }}</el-button>
      </template>
    </el-dialog>

    <!-- 分组管理弹窗 -->
    <el-dialog v-model="Groupdialog" title="分组管理" width="760px" align-center>
      <el-form label-position="top" class="node-form">
        <el-alert type="info" :closable="false" :title="`当前隐藏 ${hiddenNodeCount} 个节点。隐藏只影响全局展示、检测、推荐和订阅下发，不会删除节点或分组。`" />
        <el-divider content-position="left">显示与隐藏</el-divider>
        <el-table :data="groupDetails" size="small" max-height="220">
          <el-table-column prop="name" label="分组" min-width="180" />
          <el-table-column prop="node_count" label="节点" width="80" />
          <el-table-column label="状态" width="100"><template #default="{row}"><el-tag :type="row.hidden ? 'info' : 'success'" size="small">{{ row.hidden ? '已隐藏' : '显示' }}</el-tag></template></el-table-column>
          <el-table-column label="操作" width="110"><template #default="{row}"><el-button link :type="row.hidden ? 'primary' : 'warning'" @click="toggleGroupHidden(row)">{{ row.hidden ? '恢复' : '隐藏整组' }}</el-button></template></el-table-column>
        </el-table>
        <div class="group-node-visibility">
          <el-select v-model="manageGroupId" placeholder="选择分组查看内部节点" class="full">
            <el-option v-for="g in groupDetails" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
          <el-table :data="manageGroupNodes" size="small" max-height="260">
            <el-table-column prop="Name" label="节点" min-width="260" />
            <el-table-column label="状态" width="110"><template #default="{row}"><el-tag :type="nodeEffectiveHidden(row) ? 'info' : 'success'" size="small">{{ nodeEffectiveHidden(row) ? '全局隐藏' : '显示' }}</el-tag></template></el-table-column>
            <el-table-column label="操作" width="100"><template #default="{row}"><el-button link :type="row.Hidden ? 'primary' : 'warning'" @click="toggleNodeHidden(row)">{{ row.Hidden ? '恢复节点' : '隐藏节点' }}</el-button></template></el-table-column>
          </el-table>
          <small class="visibility-tip">如果整个分组处于隐藏状态，单独恢复节点后仍不会显示；需要先恢复分组。</small>
        </div>

        <el-divider content-position="left">绑定节点到分组</el-divider>
        <el-form-item label="选择节点">
          <el-select v-model="SelectionNode" filterable placeholder="搜索并选择节点…" class="full">
            <el-option v-for="n in overviewList" :key="n.id" :label="n.name" :value="n.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定到">
          <el-radio-group v-model="RadioGroup" size="small" class="group-tabs">
            <el-radio-button v-if="allGroupNames.length" value="1">已有分组</el-radio-button>
            <el-radio-button value="2">新分组</el-radio-button>
          </el-radio-group>
          <div v-if="RadioGroup === '1' && allGroupNames.length" class="group-field">
            <el-select v-model="SelectionNodeGroups" multiple placeholder="选择分组" class="full">
              <el-option v-for="g in allGroupNames" :key="g" :label="g" :value="g" />
            </el-select>
          </div>
          <div v-else class="group-field">
            <el-input v-model="NodeGroupInput" placeholder="输入新分组名" />
          </div>
        </el-form-item>
        <div class="group-actions">
          <el-button type="primary" @click="AddGroup">绑定</el-button>
        </div>

        <div class="del-group-card">
          <div class="del-group-title">删除分组</div>
          <div class="del-group-row">
            <el-select v-model="delGroupName" placeholder="选择要删除的分组" class="full">
              <el-option v-for="g in allGroupNames" :key="g" :label="g" :value="g" />
            </el-select>
            <el-button type="danger" @click="delGroup(delGroupName)">删除所选</el-button>
          </div>
          <div class="text-xs text-gray-400 mt-1">删除分组仅解除节点绑定，不会删除节点本身。</div>
        </div>
      </el-form>
      <template #footer>
        <el-button text @click="Groupdialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 解锁 / TCP 测试弹窗 -->
    <NodeUnlockDialog
      v-if="testNode"
      v-model:visible="unlockDialogVisible"
      :node-id="testNode.id"
      :node-name="testNode.name"
    />
    <NodeTcpDialog
      v-if="testNode"
      v-model:visible="tcpDialogVisible"
      :node-id="testNode.id"
      :node-name="testNode.name"
    />
    <NodeQualityDialog v-model:visible="qualityDialogVisible" :node="qualityNode" />
  </div>
</template>

<style scoped>
/* ===== layout ===== */
.nodes-layout { display: grid; grid-template-columns: 232px minmax(0, 1fr); gap: 16px; align-items: start; }

/* ===== group rail ===== */
.group-rail {
  position: sticky;
  top: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 14px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-lg);
  background: var(--ui-surface-strong);
  box-shadow: 0 10px 30px color-mix(in srgb, var(--ui-accent-shadow) 30%, transparent);
}
.rail-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; color: var(--ui-text); font-size: 13px; font-weight: 700; }
.rail-manage { padding: 2px 6px; border: 0; border-radius: 6px; background: transparent; color: var(--ui-accent-strong); font-size: 12px; font-weight: 600; cursor: pointer; }
.rail-manage:hover { background: var(--ui-accent-soft); }
.rail-item {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  padding: 9px 12px; border: 0; border-radius: var(--r-sm); background: transparent;
  color: var(--ui-text-secondary); font-size: 12.5px; font-weight: 600; text-align: left; cursor: pointer;
  transition: background var(--dur-fast) ease, color var(--dur-fast) ease, transform var(--dur-mid) var(--ease-spring);
}
.rail-item:hover { background: var(--ui-hover); color: var(--ui-text); transform: translateX(3px); }
.rail-item.active { background: var(--ui-accent-soft); color: var(--ui-accent-strong); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--ui-accent) 26%, transparent); }
.rail-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rail-meta { display: inline-flex; flex-shrink: 0; align-items: center; gap: 6px; }
.rail-count { color: var(--ui-text-muted); font-family: var(--font-mono); font-size: 11px; font-weight: 600; }
.rail-item.active .rail-count { color: var(--ui-accent-strong); }
.rail-hide { font-size: 10.5px; font-style: normal; color: var(--ui-text-muted); opacity: 0; transition: opacity var(--dur-fast) ease; cursor: pointer; }
.rail-item:hover .rail-hide { opacity: 1; }
.rail-hide:hover { color: var(--el-color-warning); }
.rail-foot { margin: 10px 2px 0; color: var(--ui-text-muted); font-size: 10.5px; line-height: 1.5; }

/* ===== toolbar + body ===== */
.nodes-main { min-width: 0; }
.nodes-toolbar {
  display: flex; flex-wrap: wrap; align-items: center; gap: 10px;
  padding: 14px 16px; border: 1px solid var(--ui-border); border-bottom: 0;
  border-radius: var(--r-lg) var(--r-lg) 0 0; background: var(--ui-surface-strong);
}
.toolbar-search { width: 220px; }
.toolbar-country { width: 200px; }
.view-switch { display: inline-flex; gap: 3px; padding: 3px; border: 1px solid var(--ui-border); border-radius: var(--r-sm); background: var(--ui-hover); }
.view-switch button { padding: 6px 14px; border: 0; border-radius: 6px; background: transparent; color: var(--ui-text-muted); font-size: 12px; font-weight: 600; cursor: pointer; transition: background var(--dur-fast) ease, color var(--dur-fast) ease; }
.view-switch button.active { background: var(--ui-surface-strong); color: var(--ui-text); box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08); }
.toolbar-spacer { flex: 1; }
.batch-caret { display: inline-block; width: 0; height: 0; margin-left: 6px; border-right: 4px solid transparent; border-left: 4px solid transparent; border-top: 5px solid currentcolor; opacity: 0.7; vertical-align: middle; }

.nodes-body {
  min-height: 420px;
  padding: 18px 16px 20px;
  border: 1px solid var(--ui-border);
  border-radius: 0 0 var(--r-lg) var(--r-lg);
  background: color-mix(in srgb, var(--ui-surface) 62%, var(--ui-surface-strong));
}

/* ===== country block + node cards ===== */
.country-block { margin-bottom: 20px; }
.country-head { display: flex; align-items: center; gap: 8px; margin: 0 2px 10px; }
.country-flag { font-size: 15px; }
.country-name { color: var(--ui-text); font-size: 12.5px; font-weight: 700; }
.country-count { color: var(--ui-text-muted); font-family: var(--font-mono); font-size: 11px; font-weight: 600; }
.country-select { padding: 2px 6px; border: 0; border-radius: 6px; background: transparent; color: var(--ui-accent-strong); font-size: 11px; font-weight: 600; cursor: pointer; }
.country-select:hover { background: var(--ui-accent-soft); }

.node-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.node-card {
  display: flex; flex-direction: column; gap: 9px;
  padding: 14px 14px 10px; border: 1px solid var(--ui-border); border-radius: var(--r-md);
  background: var(--ui-surface-strong);
  transition: transform var(--dur-mid) var(--ease-out), box-shadow var(--dur-mid) ease, border-color var(--dur-mid) ease;
}
.node-card:hover { transform: translateY(-3px); border-color: color-mix(in srgb, var(--ui-accent) 34%, var(--ui-border)); box-shadow: 0 16px 34px color-mix(in srgb, var(--ui-accent-shadow) 55%, transparent); }
.node-card.selected { border-color: var(--ui-accent); box-shadow: inset 0 0 0 1px var(--ui-accent), 0 10px 24px color-mix(in srgb, var(--ui-accent-shadow) 60%, transparent); }
.node-card-top { display: flex; align-items: center; gap: 8px; }
.node-flag { font-size: 16px; }
.node-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--ui-text); font-size: 13px; font-weight: 600; }
.node-meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--ui-text-muted); font-size: 11px; }
.node-stats { display: flex; align-items: center; gap: 8px; color: var(--ui-text-secondary); font-size: 11px; }
.node-rtt { padding: 2px 8px; border-radius: var(--r-pill); font-family: var(--font-mono); font-size: 11px; font-weight: 600; }
.node-rtt.good { color: #16803c; background: color-mix(in srgb, var(--el-color-success) 16%, transparent); }
.node-rtt.warn { color: #a16207; background: color-mix(in srgb, var(--el-color-warning) 20%, transparent); }
.node-rtt.bad { color: #b02a20; background: color-mix(in srgb, var(--el-color-danger) 14%, transparent); }
.node-rtt.unknown { color: var(--ui-text-muted); background: var(--ui-hover); }
.node-actions { display: flex; align-items: center; gap: 2px; padding-top: 8px; border-top: 1px solid color-mix(in srgb, var(--ui-border) 65%, transparent); }
.node-actions button { padding: 4px 8px; border: 0; border-radius: 6px; background: transparent; color: var(--ui-text-secondary); font-size: 11.5px; font-weight: 600; cursor: pointer; transition: background var(--dur-fast) ease, color var(--dur-fast) ease; }
.node-actions button:hover { background: var(--ui-hover); color: var(--ui-text); }
.node-more { margin-left: auto; }
.list-actions { display: flex; gap: 8px; margin-top: 14px; }

/* ===== kept dialog styles ===== */
.group-node-visibility { display: flex; flex-direction: column; gap: 10px; margin-top: 14px; }
.visibility-tip { color: var(--el-text-color-secondary); font-size: 11px; line-height: 1.6; }
.row-flag { margin-right: 6px; }
.row-name { font-weight: 500; }
.muted { color: var(--el-text-color-placeholder); font-size: 12px; }
.del-group-row { display: flex; gap: 10px; align-items: center; }
.node-form .el-divider { margin: 6px 0 16px; }
.node-form .el-form-item { margin-bottom: 16px; }
.group-tabs { display: block; margin-bottom: 10px; }
.group-field { margin-bottom: 4px; }
.full { width: 100%; }
.group-actions { margin-top: 4px; }
.del-group-card { margin-top: 4px; padding: 12px 14px; background: var(--el-color-danger-light-9); border: 1px solid var(--el-color-danger-light-7); border-radius: 10px; }
html.dark .del-group-card { background: rgba(224, 117, 105, 0.08); border-color: rgba(224, 117, 105, 0.2); }
.del-group-title { margin-bottom: 8px; color: var(--el-color-danger); font-size: 13px; font-weight: 600; }
.del-group-row .el-select { flex: 1; }

/* ===== responsive ===== */
@media (max-width: 900px) {
  .nodes-layout { grid-template-columns: 1fr; }
  .group-rail { position: static; flex-direction: row; flex-wrap: wrap; align-items: center; gap: 6px; padding: 12px; }
  .rail-head { width: 100%; margin: 0; }
  .rail-item { width: auto; padding: 7px 12px; }
  .rail-foot { width: 100%; margin: 2px; }
}
@media (max-width: 720px) {
  .nodes-toolbar { padding: 12px; }
  .toolbar-search, .toolbar-country { flex: 1 1 100%; width: 100%; }
  .toolbar-spacer { display: none; }
  .view-switch { flex: 1; }
  .view-switch button { flex: 1; }
  .nodes-body { padding: 12px; }
  .node-grid { grid-template-columns: 1fr 1fr; }
  .list-actions { flex-wrap: wrap; }
  .del-group-row { flex-direction: column; align-items: stretch; }
  .del-group-row :deep(.el-button) { width: 100%; margin-left: 0; }
  .group-tabs { display: flex; flex-wrap: wrap; }
  .group-node-visibility :deep(.el-table) { font-size: 12px; }
}
@media (max-width: 460px) {
  .node-grid { grid-template-columns: 1fr; }
}
</style>
