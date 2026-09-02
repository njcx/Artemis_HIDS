<template>
  <page-layout>
    <a-card :bordered="false">
      <!-- 统计信息 -->
      <div class="agent-stats">
        <a-row :gutter="16">
          <a-col :xs="24" :sm="12" :md="6">
            <div class="stat-card total">
              <div class="stat-icon">
                <a-icon type="desktop" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ data.length }}</div>
                <div class="stat-label">总主机数</div>
              </div>
            </div>
          </a-col>
          <a-col :xs="24" :sm="12" :md="6">
            <div class="stat-card online">
              <div class="stat-icon">
                <a-icon type="check-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ onlineCount }}</div>
                <div class="stat-label">在线主机</div>
              </div>
            </div>
          </a-col>
          <a-col :xs="24" :sm="12" :md="6">
            <div class="stat-card offline">
              <div class="stat-icon">
                <a-icon type="close-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ offlineCount }}</div>
                <div class="stat-label">离线主机</div>
              </div>
            </div>
          </a-col>
          <a-col :xs="24" :sm="12" :md="6">
            <div class="stat-card rate">
              <div class="stat-icon">
                <a-icon type="pie-chart" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ onlineRate.toFixed(1) }}%</div>
                <div class="stat-label">在线率</div>
              </div>
            </div>
          </a-col>
        </a-row>
      </div>

      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="8" :sm="24">
              <a-form-item label="主机名/IP">
                <a-input v-model="queryParams.search" placeholder="请输入主机名或IP地址" />
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-form-item label="状态">
                <a-select v-model="queryParams.status" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="online">在线</a-select-option>
                  <a-select-option value="offline">离线</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-button type="primary" @click="handleSearch">查询</a-button>
              <a-button style="margin-left: 8px" @click="handleReset">重置</a-button>
            </a-col>
          </a-row>
        </a-form>
      </div>

      <div class="table-operator">
        <a-button type="primary" icon="reload" @click="handleRefresh">刷新</a-button>
        <a-button
          icon="cloud-upload"
          :disabled="selectedRowKeys.length === 0"
          @click="handleBatchUpgrade"
        >批量升级</a-button>
        <a-button
          icon="delete"
          :disabled="selectedRowKeys.length === 0"
          @click="handleBatchUninstall"
        >批量卸载</a-button>
        <span v-if="selectedRowKeys.length > 0" class="selected-hint">
          已选择 {{ selectedRowKeys.length }} 个主机
        </span>
      </div>

      <a-table
        :columns="columns"
        :data-source="data"
        :pagination="pagination"
        :loading="loading"
        :row-selection="{ selectedRowKeys: selectedRowKeys, onChange: onSelectChange }"
        @change="handleTableChange"
        rowKey="id"
      >
        <template slot="status" slot-scope="status">
          <a-badge :status="status === 'online' ? 'success' : 'error'" :text="status === 'online' ? '在线' : '离线'" />
        </template>

        <template slot="os" slot-scope="os">
          <a-tag :color="getOSColor(os)">
            <a-icon :type="getOSIcon(os)" />
            {{ os }}
          </a-tag>
        </template>

        <template slot="lastSeen" slot-scope="lastSeen">
          {{ formatTime(lastSeen) }}
        </template>

        <template slot="action" slot-scope="text, record">
          <a @click="handleView(record)">详情</a>
          <a-divider type="vertical" />
          <a-dropdown>
            <a class="ant-dropdown-link">
              更多 <a-icon type="down" />
            </a>
            <a-menu slot="overlay">
              <a-menu-item>
                <a @click="handleScan(record)">执行扫描</a>
              </a-menu-item>
              <a-menu-item>
                <a @click="handleRestart(record)">重启Agent</a>
              </a-menu-item>
              <a-menu-item>
                <a @click="handleUpgrade(record)">升级</a>
              </a-menu-item>
              <a-menu-item>
                <a-popconfirm
                  title="确定要卸载该 Agent 吗？该操作不可恢复！"
                  @confirm="handleUninstall(record)"
                >
                  <a style="color: #f5222d">卸载</a>
                </a-popconfirm>
              </a-menu-item>
              <a-menu-item>
                <a style="color: #f5222d" @click="handleDelete(record)">删除</a>
              </a-menu-item>
            </a-menu>
          </a-dropdown>
        </template>
      </a-table>
    </a-card>

    <!-- 升级弹窗（单个/批量共用） -->
    <a-modal
      title="升级 Agent"
      :visible="upgradeVisible"
      :confirm-loading="upgradeLoading"
      @ok="handleUpgradeOk"
      @cancel="upgradeVisible = false"
      width="600px"
    >
      <p style="margin-bottom: 16px;">
        即将升级 <strong>{{ upgradeTargetIds.length }}</strong> 个主机的 Agent
      </p>
      <a-form-model
        ref="upgradeForm"
        :model="upgradeForm"
        :rules="upgradeRules"
        :label-col="{ span: 6 }"
        :wrapper-col="{ span: 16 }"
      >
        <a-form-model-item label="目标版本" prop="version">
          <a-input v-model="upgradeForm.version" placeholder="如 1.2.0" />
        </a-form-model-item>
        <a-form-model-item label="下载地址" prop="download_url">
          <a-input v-model="upgradeForm.download_url" placeholder="新版本二进制下载 URL" />
        </a-form-model-item>
        <a-form-model-item label="SHA256 校验值" prop="checksum">
          <a-input v-model="upgradeForm.checksum" placeholder="用于校验下载文件完整性" />
        </a-form-model-item>
      </a-form-model>
    </a-modal>

    <!-- 批量卸载确认弹窗 -->
    <a-modal
      title="批量卸载 Agent"
      :visible="batchUninstallVisible"
      :confirm-loading="batchUninstallLoading"
      :ok-button-props="{ props: { disabled: batchUninstallConfirmText !== 'CONFIRM' } }"
      @ok="handleBatchUninstallOk"
      @cancel="batchUninstallVisible = false"
    >
      <p>
        即将卸载 <strong>{{ selectedRowKeys.length }}</strong> 个主机的 Agent，
        <span style="color: #f5222d">该操作不可恢复</span>。
      </p>
      <p>请输入 <strong>CONFIRM</strong> 以确认执行：</p>
      <a-input v-model="batchUninstallConfirmText" placeholder="请输入 CONFIRM" />
    </a-modal>

    <!-- 命令执行结果轮询弹窗 -->
    <a-modal
      title="命令执行结果"
      :visible="commandResultVisible"
      :footer="null"
      @cancel="closeCommandResultModal"
      width="600px"
    >
      <p v-if="commandResultPolling" style="color: #888;">
        <a-icon type="loading" /> 正在等待 Agent 上报执行结果（最长等待 60 秒）...
      </p>
      <p v-else style="color: #888;">已停止等待，未完成的命令可能仍在 Agent 端异步执行。</p>
      <a-table
        :columns="commandResultColumns"
        :data-source="commandResultList"
        :pagination="false"
        rowKey="agent_id"
        size="small"
      >
        <template slot="status" slot-scope="status">
          <a-tag v-if="status === 'pending'" color="blue"><a-icon type="loading" /> 执行中</a-tag>
          <a-tag v-else-if="status === 'success'" color="green"><a-icon type="check" /> 成功</a-tag>
          <a-tag v-else-if="status === 'failed'" color="red"><a-icon type="close" /> 失败</a-tag>
          <a-tag v-else color="orange">超时未确认</a-tag>
        </template>
      </a-table>
    </a-modal>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import {request, METHOD} from '@/utils/request'
import { updateAgent, uninstallAgent, batchCommand, getCommandResult } from '@/services/agentManagement'

const COMMAND_RESULT_POLL_INTERVAL = 2000
const COMMAND_RESULT_TIMEOUT = 60000

export default {
  name: 'AgentList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        search: '',
        status: ''
      },
      loading: false,
      data: [],
      selectedRowKeys: [],
      columns: [
        {
          title: 'Agent ID',
          dataIndex: 'agent_id',
          key: 'agent_id',
          width: 180
        },
        {
          title: 'IP地址',
          dataIndex: 'ip',
          key: 'ip',
          width: 140
        },
        {
          title: '发行版',
          dataIndex: 'os',
          key: 'os',
          width: 180,
          scopedSlots: { customRender: 'os' }
        },
        {
          title: '状态',
          dataIndex: 'status',
          key: 'status',
          width: 100,
          scopedSlots: { customRender: 'status' }
        },
        {
          title: '最后心跳',
          dataIndex: 'lastSeen',
          key: 'lastSeen',
          width: 150,
          scopedSlots: { customRender: 'lastSeen' }
        },
        {
          title: '操作',
          key: 'action',
          width: 200,
          scopedSlots: { customRender: 'action' }
        }
      ],
      pagination: {
        current: 1,
        pageSize: 10,
        total: 0,
        showSizeChanger: true,
        showQuickJumper: true,
        showTotal: total => `共 ${total} 条记录`
      },

      // 升级弹窗
      upgradeVisible: false,
      upgradeLoading: false,
      upgradeTargetIds: [],
      upgradeForm: {
        version: '',
        download_url: '',
        checksum: ''
      },
      upgradeRules: {
        version: [{ required: true, message: '请输入目标版本', trigger: 'blur' }],
        download_url: [{ required: true, message: '请输入下载地址', trigger: 'blur' }],
        checksum: [{ required: true, message: '请输入 SHA256 校验值', trigger: 'blur' }]
      },

      // 批量卸载弹窗
      batchUninstallVisible: false,
      batchUninstallLoading: false,
      batchUninstallConfirmText: '',

      // 命令结果轮询弹窗
      commandResultVisible: false,
      commandResultPolling: false,
      commandResultList: [],
      commandResultColumns: [
        { title: 'Agent ID', dataIndex: 'agent_id', key: 'agent_id' },
        { title: '状态', dataIndex: 'status', key: 'status', scopedSlots: { customRender: 'status' } },
        { title: '说明', dataIndex: 'message', key: 'message' }
      ],
      commandResultTimer: null,
      commandResultStartedAt: 0
    }
  },
  mounted() {
    this.loadAgents()
  },
  beforeDestroy() {
    this.stopCommandResultPolling()
  },
  computed: {
    onlineCount() {
      return this.data.filter(item => item.status === 'online').length
    },
    offlineCount() {
      return this.data.filter(item => item.status === 'offline').length
    },
    onlineRate() {
      if (this.data.length === 0) return 0
      return (this.onlineCount / this.data.length) * 100
    }
  },
  methods: {
    async loadAgents() {
      this.loading = true
      try {
        const response = await request('/api/v1/agents', METHOD.GET)
        const agents = response.data.agents || []

        // 转换数据格式
        this.data = agents.map(agent => ({
          id: agent.agent_id,
          agent_id: agent.agent_id || 'N/A',
          hostname: agent.hostname || 'N/A',
          ip: agent.ip || 'N/A',
          os: this.formatOS(agent),
          status: agent.online ? 'online' : 'offline',
          lastSeen: this.parseLastHeartbeat(agent.last_heartbeat)
        }))
        this.pagination.total = this.data.length
        this.$message.success('主机列表加载成功')
      } catch (error) {
        console.error('加载主机列表失败:', error)
        this.$message.error('加载主机列表失败: ' + (error.response?.data?.error || error.message))
      } finally {
        this.loading = false
      }
    },
    formatOS(agent) {
      // 组合发行版和版本号，如 "centos 7" 或 "ubuntu 20.04"
      const parts = []
      // 优先使用 platform（发行版名称），如 centos, ubuntu
      if (agent.platform) {
        parts.push(agent.platform.toLowerCase())
      } else if (agent.os) {
        parts.push(agent.os.toLowerCase())
      }
      // 添加版本号
      if (agent.version) {
        // 提取主版本号，如 "7.9.2009" -> "7" 或 "20.04.1" -> "20.04"
        const versionMatch = agent.version.match(/^(\d+(?:\.\d+)?)/)
        if (versionMatch) {
          parts.push(versionMatch[1])
        }
      }
      return parts.length > 0 ? parts.join(' ') : 'N/A'
    },
    parseLastHeartbeat(lastHeartbeat) {
      // 解析最后心跳时间，支持多种格式
      if (!lastHeartbeat) {
        return new Date()
      }

      // 如果是字符串格式（ISO 8601）
      if (typeof lastHeartbeat === 'string') {
        const date = new Date(lastHeartbeat)
        if (!isNaN(date.getTime())) {
          return date
        }
      }

      // 如果是数字格式
      if (typeof lastHeartbeat === 'number') {
        // 判断是秒还是毫秒（Unix 时间戳通常是秒，JavaScript 使用毫秒）
        // 如果数值大于 1e12，认为是毫秒；否则是秒
        if (lastHeartbeat > 1e12) {
          return new Date(lastHeartbeat)
        } else {
          return new Date(lastHeartbeat * 1000)
        }
      }

      // 如果是对象格式（可能是 MongoDB 的日期对象）
      if (typeof lastHeartbeat === 'object' && lastHeartbeat !== null) {
        // 尝试获取 $date 字段（MongoDB Extended JSON 格式）
        if (lastHeartbeat.$date) {
          return new Date(lastHeartbeat.$date)
        }
        // 尝试直接转换
        const date = new Date(lastHeartbeat)
        if (!isNaN(date.getTime())) {
          return date
        }
      }

      // 默认返回当前时间
      return new Date()
    },
    handleSearch() {
      this.loading = true
      setTimeout(() => {
        this.loading = false
        this.$message.success('查询成功')
      }, 500)
    },
    handleReset() {
      this.queryParams = {
        search: '',
        status: ''
      }
    },
    handleRefresh() {
      this.loadAgents()
    },
    handleAdd() {
      this.$message.info('添加主机功能')
    },
    handleView(record) {
      // 跳转到主机详情页面
      this.$router.push({
        name: '主机详情',
        params: { id: record.id }
      })
    },
    handleEdit(record) {
      this.$message.info(`编辑主机: ${record.hostname}`)
    },
    handleScan(record) {
      this.$message.info(`执行扫描: ${record.hostname}`)
    },
    handleRestart(record) {
      this.$message.info(`重启Agent: ${record.hostname}`)
    },
    handleDelete(record) {
      this.$confirm({
        title: '确认删除',
        content: `确定要删除主机 ${record.hostname} 吗？`,
        onOk: () => {
          this.$message.success('删除成功')
        }
      })
    },
    handleTableChange(pagination) {
      this.pagination = pagination
      this.handleSearch()
    },
    onSelectChange(selectedRowKeys) {
      this.selectedRowKeys = selectedRowKeys
    },

    // ============ 升级 ============
    handleUpgrade(record) {
      this.upgradeTargetIds = [record.id]
      this.upgradeForm = { version: '', download_url: '', checksum: '' }
      this.upgradeVisible = true
    },
    handleBatchUpgrade() {
      this.upgradeTargetIds = [...this.selectedRowKeys]
      this.upgradeForm = { version: '', download_url: '', checksum: '' }
      this.upgradeVisible = true
    },
    handleUpgradeOk() {
      this.$refs.upgradeForm.validate(async valid => {
        if (!valid) return

        this.upgradeLoading = true
        try {
          const payload = {
            version: this.upgradeForm.version,
            download_url: this.upgradeForm.download_url,
            checksum: this.upgradeForm.checksum
          }

          let items = []
          if (this.upgradeTargetIds.length === 1) {
            const res = await updateAgent(this.upgradeTargetIds[0], payload)
            items = [{ agent_id: this.upgradeTargetIds[0], command_id: res.data.command_id }]
          } else {
            const res = await batchCommand(this.upgradeTargetIds, 'update_agent', payload)
            items = (res.data.results || []).map(r => ({ agent_id: r.agent_id, command_id: r.command_id }))
          }

          this.$message.success('升级命令已下发')
          this.upgradeVisible = false
          this.openCommandResultModal(items)
        } catch (error) {
          console.error('下发升级命令失败:', error)
          this.$message.error(error.response?.data?.error || '下发升级命令失败')
        } finally {
          this.upgradeLoading = false
        }
      })
    },

    // ============ 卸载 ============
    async handleUninstall(record) {
      try {
        const res = await uninstallAgent(record.id)
        this.$message.success('卸载命令已下发，Agent 将在下次心跳后执行')
        this.openCommandResultModal([{ agent_id: record.id, command_id: res.data.command_id }])
      } catch (error) {
        console.error('下发卸载命令失败:', error)
        this.$message.error(error.response?.data?.error || '下发卸载命令失败')
      }
    },
    handleBatchUninstall() {
      this.batchUninstallConfirmText = ''
      this.batchUninstallVisible = true
    },
    async handleBatchUninstallOk() {
      if (this.batchUninstallConfirmText !== 'CONFIRM') {
        this.$message.warning('请输入 CONFIRM 以确认执行')
        return
      }

      this.batchUninstallLoading = true
      try {
        const res = await batchCommand(this.selectedRowKeys, 'uninstall_agent', {})
        const items = (res.data.results || []).map(r => ({ agent_id: r.agent_id, command_id: r.command_id }))

        this.$message.success('批量卸载命令已下发')
        this.batchUninstallVisible = false
        this.openCommandResultModal(items)
      } catch (error) {
        console.error('下发批量卸载命令失败:', error)
        this.$message.error(error.response?.data?.error || '下发批量卸载命令失败')
      } finally {
        this.batchUninstallLoading = false
      }
    },

    // ============ 命令执行结果轮询 ============
    openCommandResultModal(items) {
      this.stopCommandResultPolling()

      this.commandResultList = items
        .filter(item => item.command_id)
        .map(item => ({ ...item, status: 'pending', message: '' }))

      if (this.commandResultList.length === 0) {
        this.$message.warning('没有可跟踪的命令，请稍后到命令历史中查看')
        return
      }

      this.commandResultVisible = true
      this.commandResultPolling = true
      this.commandResultStartedAt = Date.now()

      this.pollCommandResults()
      this.commandResultTimer = setInterval(() => this.pollCommandResults(), COMMAND_RESULT_POLL_INTERVAL)
    },
    async pollCommandResults() {
      const pendingItems = this.commandResultList.filter(item => item.status === 'pending')

      if (pendingItems.length === 0) {
        this.stopCommandResultPolling()
        return
      }

      if (Date.now() - this.commandResultStartedAt > COMMAND_RESULT_TIMEOUT) {
        this.stopCommandResultPolling()
        return
      }

      await Promise.all(pendingItems.map(async item => {
        try {
          const res = await getCommandResult(item.command_id, item.agent_id)
          const result = res.data
          item.status = result.success ? 'success' : 'failed'
          item.message = result.message || ''
        } catch (error) {
          // 404 表示 Agent 还没上报结果，继续等待；其他错误也先当作继续等待处理
        }
      }))

      // 触发视图更新（数组内对象属性变更，Vue2 需要重新赋值才能保证响应性）
      this.commandResultList = [...this.commandResultList]

      if (this.commandResultList.every(item => item.status !== 'pending')) {
        this.stopCommandResultPolling()
      }
    },
    stopCommandResultPolling() {
      if (this.commandResultTimer) {
        clearInterval(this.commandResultTimer)
        this.commandResultTimer = null
      }
      this.commandResultPolling = false
    },
    closeCommandResultModal() {
      this.stopCommandResultPolling()
      this.commandResultVisible = false
    },

    getOSColor(os) {
      const osLower = os.toLowerCase()
      if (osLower.includes('ubuntu') || osLower.includes('debian')) return 'orange'
      if (osLower.includes('centos') || osLower.includes('redhat') || osLower.includes('rhel')) return 'purple'
      if (osLower.includes('windows')) return 'blue'
      if (osLower.includes('linux')) return 'green'
      return 'cyan' // 默认使用青色，确保可见
    },
    getOSIcon(os) {
      const osLower = os.toLowerCase()
      if (osLower.includes('windows')) return 'windows'
      // Linux 系统使用 desktop 图标（没有专门的 Linux 图标）
      return 'desktop'
    },
    formatTime(time) {
      const now = new Date()
      const diff = Math.floor((now - time) / 1000)

      if (diff < 60) return '刚刚'
      if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
      if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
      return new Date(time).toLocaleString()
    }
  }
}
</script>

<style lang="less" scoped>
.agent-stats {
  margin-bottom: 24px;

  .stat-card {
    display: flex;
    align-items: center;
    padding: 20px;
    border-radius: 8px;
    background: #fff;
    border: 1px solid #f0f0f0;
    transition: all 0.3s ease;

    &:hover {
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
      transform: translateY(-2px);
    }

    .stat-icon {
      width: 56px;
      height: 56px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 8px;
      margin-right: 16px;

      .anticon {
        font-size: 28px;
        color: #fff;
      }
    }

    .stat-content {
      flex: 1;

      .stat-value {
        font-size: 28px;
        font-weight: 600;
        line-height: 1;
        margin-bottom: 4px;
      }

      .stat-label {
        font-size: 14px;
        color: #8c8c8c;
      }
    }

    &.total {
      .stat-icon {
        background: linear-gradient(135deg, #1890ff 0%, #40a9ff 100%);
      }
      .stat-value {
        color: #1890ff;
      }
    }

    &.online {
      .stat-icon {
        background: linear-gradient(135deg, #52c41a 0%, #73d13d 100%);
      }
      .stat-value {
        color: #52c41a;
      }
    }

    &.offline {
      .stat-icon {
        background: linear-gradient(135deg, #ff4d4f 0%, #ff7875 100%);
      }
      .stat-value {
        color: #ff4d4f;
      }
    }

    &.rate {
      .stat-icon {
        background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);
      }
      .stat-value {
        color: #722ed1;
      }
    }
  }
}

.table-page-search-wrapper {
  margin-bottom: 16px;
}

.table-operator {
  margin-bottom: 16px;

  button {
    margin-right: 8px;
  }

  .selected-hint {
    margin-left: 8px;
    color: #888;
  }
}
</style>
