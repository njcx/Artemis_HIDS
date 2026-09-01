<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="8" :sm="24">
              <a-form-item label="组件名称">
                <a-input v-model="queryParams.name" placeholder="请输入组件名称" />
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-form-item label="状态">
                <a-select v-model="queryParams.status" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="running">运行中</a-select-option>
                  <a-select-option value="stopped">已停止</a-select-option>
                  <a-select-option value="error">异常</a-select-option>
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

      <div class="component-stats">
        <a-row :gutter="16">
          <a-col :span="6">
            <div class="stat-card running">
              <div class="stat-icon">
                <a-icon type="check-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.running }}</div>
                <div class="stat-label">运行中</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card stopped">
              <div class="stat-icon">
                <a-icon type="pause-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.stopped }}</div>
                <div class="stat-label">已停止</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card error">
              <div class="stat-icon">
                <a-icon type="close-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.error }}</div>
                <div class="stat-label">异常</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card total">
              <div class="stat-icon">
                <a-icon type="appstore" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.total }}</div>
                <div class="stat-label">总组件数</div>
              </div>
            </div>
          </a-col>
        </a-row>
      </div>

      <a-table
        :columns="columns"
        :data-source="data"
        :pagination="pagination"
        :loading="loading"
        @change="handleTableChange"
        rowKey="id"
      >
        <template slot="name" slot-scope="text, record">
          <div class="component-name">
            <a-icon :type="getComponentIcon(record.type)" style="margin-right: 8px; font-size: 18px;" />
            <span>{{ text }}</span>
          </div>
        </template>

        <template slot="type" slot-scope="type">
          <a-tag :color="getTypeColor(type)">
            {{ getTypeText(type) }}
          </a-tag>
        </template>
        
        <template slot="status" slot-scope="status">
          <a-badge :status="getStatusBadge(status)" :text="getStatusText(status)" />
        </template>
        
        <template slot="cpu" slot-scope="cpu">
          <a-progress :percent="cpu" size="small" :status="cpu > 80 ? 'exception' : 'normal'" style="width: 120px" />
        </template>
        
        <template slot="memory" slot-scope="memory">
          <span>{{ memory }} MB</span>
        </template>
        
        <template slot="action" slot-scope="text, record">
          <a @click="handleView(record)">详情</a>
          <a-divider type="vertical" />
          <a-dropdown>
            <a class="ant-dropdown-link">
              操作 <a-icon type="down" />
            </a>
            <a-menu slot="overlay">
              <a-menu-item v-if="record.status === 'stopped'">
                <a @click="handleStart(record)">
                  <a-icon type="play-circle" /> 启动
                </a>
              </a-menu-item>
              <a-menu-item v-if="record.status === 'running'">
                <a @click="handleStop(record)">
                  <a-icon type="pause-circle" /> 停止
                </a>
              </a-menu-item>
              <a-menu-item v-if="record.status === 'running'">
                <a @click="handleRestart(record)">
                  <a-icon type="reload" /> 重启
                </a>
              </a-menu-item>
              <a-menu-item>
                <a @click="handleConfig(record)">
                  <a-icon type="setting" /> 配置
                </a>
              </a-menu-item>
              <a-menu-divider />
              <a-menu-item>
                <a @click="handleLogs(record)">
                  <a-icon type="file-text" /> 查看日志
                </a>
              </a-menu-item>
            </a-menu>
          </a-dropdown>
        </template>
      </a-table>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'

export default {
  name: 'ComponentList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        name: '',
        status: ''
      },
      loading: false,
      stats: {
        running: 5,
        stopped: 1,
        error: 1,
        total: 7
      },
      data: [
        {
          id: 1,
          name: '文件监控模块',
          type: 'monitor',
          version: '1.0.0',
          status: 'running',
          cpu: 25,
          memory: 128,
          uptime: '5天3小时',
          description: '监控文件系统变化，检测异常文件操作'
        },
        {
          id: 2,
          name: '进程监控模块',
          type: 'monitor',
          version: '1.0.0',
          status: 'running',
          cpu: 18,
          memory: 96,
          uptime: '5天3小时',
          description: '监控进程启动、退出和异常行为'
        },
        {
          id: 3,
          name: '网络监控模块',
          type: 'monitor',
          version: '1.0.0',
          status: 'running',
          cpu: 32,
          memory: 156,
          uptime: '5天3小时',
          description: '监控网络连接和异常流量'
        },
        {
          id: 4,
          name: '规则引擎',
          type: 'engine',
          version: '2.1.0',
          status: 'running',
          cpu: 45,
          memory: 512,
          uptime: '5天3小时',
          description: '执行安全检测规则和威胁分析'
        },
        {
          id: 5,
          name: '日志收集器',
          type: 'collector',
          version: '1.2.0',
          status: 'running',
          cpu: 15,
          memory: 64,
          uptime: '5天3小时',
          description: '收集和处理系统日志'
        },
        {
          id: 6,
          name: '告警通知模块',
          type: 'notifier',
          version: '1.0.5',
          status: 'stopped',
          cpu: 0,
          memory: 0,
          uptime: '-',
          description: '发送安全告警通知'
        },
        {
          id: 7,
          name: '数据分析引擎',
          type: 'engine',
          version: '1.5.2',
          status: 'error',
          cpu: 0,
          memory: 0,
          uptime: '-',
          description: '执行安全数据分析和威胁情报关联'
        }
      ],
      columns: [
        {
          title: '组件名称',
          dataIndex: 'name',
          key: 'name',
          width: 200,
          scopedSlots: { customRender: 'name' }
        },
        {
          title: '类型',
          dataIndex: 'type',
          key: 'type',
          width: 120,
          scopedSlots: { customRender: 'type' }
        },
        {
          title: '版本',
          dataIndex: 'version',
          key: 'version',
          width: 100
        },
        {
          title: '状态',
          dataIndex: 'status',
          key: 'status',
          width: 100,
          scopedSlots: { customRender: 'status' }
        },
        {
          title: 'CPU使用率',
          dataIndex: 'cpu',
          key: 'cpu',
          width: 150,
          scopedSlots: { customRender: 'cpu' }
        },
        {
          title: '内存',
          dataIndex: 'memory',
          key: 'memory',
          width: 100,
          scopedSlots: { customRender: 'memory' }
        },
        {
          title: '运行时长',
          dataIndex: 'uptime',
          key: 'uptime',
          width: 120
        },
        {
          title: '操作',
          key: 'action',
          width: 150,
          scopedSlots: { customRender: 'action' }
        }
      ],
      pagination: {
        current: 1,
        pageSize: 10,
        total: 7,
        showSizeChanger: true,
        showQuickJumper: true,
        showTotal: total => `共 ${total} 个组件`
      }
    }
  },
  methods: {
    handleSearch() {
      this.loading = true
      setTimeout(() => {
        this.loading = false
        this.$message.success('查询成功')
      }, 500)
    },
    handleReset() {
      this.queryParams = {
        name: '',
        status: ''
      }
    },
    handleView(record) {
      this.$message.info(`查看组件详情: ${record.name}`)
    },
    handleStart(record) {
      this.$message.success(`正在启动组件: ${record.name}`)
    },
    handleStop(record) {
      this.$confirm({
        title: '停止组件',
        content: `确定要停止组件 ${record.name} 吗？`,
        onOk: () => {
          this.$message.success(`组件已停止: ${record.name}`)
        }
      })
    },
    handleRestart(record) {
      this.$message.success(`正在重启组件: ${record.name}`)
    },
    handleConfig(record) {
      this.$message.info(`配置组件: ${record.name}`)
    },
    handleLogs(record) {
      this.$message.info(`查看日志: ${record.name}`)
    },
    handleTableChange(pagination) {
      this.pagination = pagination
      this.handleSearch()
    },
    getComponentIcon(type) {
      const icons = {
        monitor: 'eye',
        engine: 'api',
        collector: 'database',
        notifier: 'bell'
      }
      return icons[type] || 'appstore'
    },
    getTypeColor(type) {
      const colors = {
        monitor: 'blue',
        engine: 'purple',
        collector: 'green',
        notifier: 'orange'
      }
      return colors[type] || 'default'
    },
    getTypeText(type) {
      const texts = {
        monitor: '监控模块',
        engine: '引擎',
        collector: '收集器',
        notifier: '通知器'
      }
      return texts[type] || type
    },
    getStatusBadge(status) {
      const badges = {
        running: 'success',
        stopped: 'default',
        error: 'error'
      }
      return badges[status] || 'default'
    },
    getStatusText(status) {
      const texts = {
        running: '运行中',
        stopped: '已停止',
        error: '异常'
      }
      return texts[status] || status
    }
  }
}
</script>

<style lang="less" scoped>
.table-page-search-wrapper {
  margin-bottom: 16px;
}

.component-stats {
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
    
    &.running {
      .stat-icon {
        background: linear-gradient(135deg, #52c41a 0%, #73d13d 100%);
      }
      .stat-value {
        color: #52c41a;
      }
    }
    
    &.stopped {
      .stat-icon {
        background: linear-gradient(135deg, #bfbfbf 0%, #d9d9d9 100%);
      }
      .stat-value {
        color: #8c8c8c;
      }
    }
    
    &.error {
      .stat-icon {
        background: linear-gradient(135deg, #ff4d4f 0%, #ff7875 100%);
      }
      .stat-value {
        color: #ff4d4f;
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
  }
}

.component-name {
  display: flex;
  align-items: center;
  font-weight: 500;
}
</style>
