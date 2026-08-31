<template>
  <page-layout>
    <a-card :bordered="false">
      <!-- 安全告警 -->
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="8" :sm="24">
              <a-form-item label="告警级别">
                <a-select v-model="alertQuery.severity" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="critical">严重</a-select-option>
                  <a-select-option value="high">高危</a-select-option>
                  <a-select-option value="medium">中危</a-select-option>
                  <a-select-option value="low">低危</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-form-item label="处理状态">
                <a-select v-model="alertQuery.status" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="pending">待处理</a-select-option>
                  <a-select-option value="investigating">调查中</a-select-option>
                  <a-select-option value="resolved">已解决</a-select-option>
                  <a-select-option value="ignored">已忽略</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-button type="primary" @click="handleAlertSearch">查询</a-button>
              <a-button style="margin-left: 8px" @click="handleAlertReset">重置</a-button>
            </a-col>
          </a-row>
        </a-form>
      </div>

      <div class="alert-stats">
        <a-row :gutter="16">
          <a-col :span="6">
            <div class="stat-card critical">
              <div class="stat-icon">
                <a-icon type="close-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.critical_alerts }}</div>
                <div class="stat-label">严重告警</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card high">
              <div class="stat-icon">
                <a-icon type="exclamation-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.high_alerts }}</div>
                <div class="stat-label">高危告警</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card medium">
              <div class="stat-icon">
                <a-icon type="warning" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.medium_alerts }}</div>
                <div class="stat-label">中危告警</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card pending">
              <div class="stat-icon">
                <a-icon type="clock-circle" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.pending_alerts }}</div>
                <div class="stat-label">待处理</div>
              </div>
            </div>
          </a-col>
        </a-row>
      </div>

      <!-- 告警列表表格 -->
      <a-table
        :columns="alertColumns"
        :data-source="alerts"
        :pagination="alertPagination"
        :loading="alertLoading"
        @change="handleAlertTableChange"
        rowKey="id"
      >
        <template slot="severity" slot-scope="text">
          <a-tag :color="getSeverityColor(text)">
            <a-icon :type="getSeverityIcon(text)" />
            {{ getSeverityText(text) }}
          </a-tag>
        </template>
        
        <template slot="status" slot-scope="text">
          <a-badge :status="getStatusBadge(text)" :text="getStatusText(text)" />
        </template>
        
        <template slot="timestamp" slot-scope="text">
          {{ formatTime(text) }}
        </template>
        
        <template slot="action" slot-scope="text, record">
          <a @click="handleAlertView(record)">详情</a>
          <a-divider type="vertical" />
          <a-dropdown v-if="record.status === 'open' || record.status === 'investigating'">
            <a class="ant-dropdown-link">
              处理 <a-icon type="down" />
            </a>
            <a-menu slot="overlay" @click="({key}) => handleStatusChange(record, key)">
              <a-menu-item key="investigating" v-if="record.status === 'open'">标记为调查中</a-menu-item>
              <a-menu-item key="resolved">标记为已解决</a-menu-item>
              <a-menu-item key="ignored">忽略告警</a-menu-item>
            </a-menu>
          </a-dropdown>
          <span v-else>-</span>
        </template>
      </a-table>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import { getAlerts, updateAlertStatus } from '@/services/alert'
import { getDashboardStats } from '@/services/stats'

export default {
  name: 'SecurityMonitor',
  components: { PageLayout },
  data() {
    return {
      stats: {
        critical_alerts: 0,
        high_alerts: 0,
        medium_alerts: 0,
        pending_alerts: 0
      },
      alertQuery: {
        severity: '',
        status: ''
      },
      alertLoading: false,
      alerts: [],
      alertColumns: [
        {
          title: '级别',
          dataIndex: 'severity',
          key: 'severity',
          width: 100,
          scopedSlots: { customRender: 'severity' }
        },
        {
          title: '告警标题',
          dataIndex: 'title',
          key: 'title',
          ellipsis: true
        },
        {
          title: '主机',
          dataIndex: 'agent_id',
          key: 'agent_id',
          ellipsis: true
        },
        {
          title: '状态',
          dataIndex: 'status',
          key: 'status',
          width: 100,
          scopedSlots: { customRender: 'status' }
        },
        {
          title: '时间',
          dataIndex: 'timestamp',
          key: 'timestamp',
          width: 180,
          scopedSlots: { customRender: 'timestamp' }
        },
        {
          title: '操作',
          key: 'action',
          width: 150,
          scopedSlots: { customRender: 'action' }
        }
      ],
      alertPagination: {
        current: 1,
        pageSize: 20,
        total: 0,
        showSizeChanger: true,
        showQuickJumper: true,
        showTotal: total => `共 ${total} 条记录`
      }
    }
  },
  mounted() {
    this.fetchStats()
    this.fetchAlerts()
  },
  methods: {
    async fetchStats() {
      try {
        const res = await getDashboardStats()
        if (res && res.data) {
          this.stats = {
            critical_alerts: res.data.critical_alerts || 0,
            high_alerts: res.data.high_alerts || 0,
            medium_alerts: res.data.medium_alerts || 0,
            pending_alerts: res.data.pending_alerts || 0
          }
        }
      } catch (error) {
        console.error('获取统计数据失败:', error)
      }
    },
    async fetchAlerts() {
      this.alertLoading = true
      try {
        const params = {
          severity: this.alertQuery.severity || 'all',
          status: this.alertQuery.status || 'all',
          page: this.alertPagination.current,
          page_size: this.alertPagination.pageSize
        }
        const res = await getAlerts(params)
        if (res && res.data) {
          this.alerts = res.data.items || []
          this.alertPagination.total = res.data.total || 0
        }
      } catch (error) {
        this.$message.error('获取告警列表失败')
        console.error(error)
      } finally {
        this.alertLoading = false
      }
    },
    handleAlertSearch() {
      this.alertPagination.current = 1
      this.fetchAlerts()
    },
    handleAlertReset() {
      this.alertQuery = {
        severity: '',
        status: ''
      }
      this.alertPagination.current = 1
      this.fetchAlerts()
    },
    handleAlertView(record) {
      // 跳转到告警详情页
      this.$router.push({
        path: `/security/${record.id}`
      })
    },
    async handleStatusChange(record, status) {
      try {
        await updateAlertStatus(record.id, { status })
        this.$message.success(`已将告警状态更新为: ${this.getStatusText(status)}`)
        this.fetchAlerts()
        this.fetchStats()
      } catch (error) {
        this.$message.error('更新告警状态失败')
        console.error(error)
      }
    },
    handleAlertTableChange(pagination) {
      this.alertPagination = pagination
      this.fetchAlerts()
    },
    getSeverityColor(severity) {
      const colors = {
        critical: 'red',
        high: 'orange',
        medium: 'gold',
        low: 'blue'
      }
      return colors[severity] || 'default'
    },
    getSeverityIcon(severity) {
      const icons = {
        critical: 'close-circle',
        high: 'exclamation-circle',
        medium: 'warning',
        low: 'info-circle'
      }
      return icons[severity] || 'info-circle'
    },
    getSeverityText(severity) {
      const texts = {
        critical: '严重',
        high: '高危',
        medium: '中危',
        low: '低危'
      }
      return texts[severity] || severity
    },
    getStatusBadge(status) {
      const badges = {
        open: 'error',
        resolved: 'success',
        ignored: 'default',
        investigating: 'processing'
      }
      return badges[status] || 'default'
    },
    getStatusText(status) {
      const texts = {
        open: '待处理',
        resolved: '已解决',
        ignored: '已忽略',
        investigating: '调查中'
      }
      return texts[status] || status
    },
    formatTime(time) {
      if (!time) return 'N/A'
      return new Date(time).toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })
    }
  }
}
</script>

<style lang="less" scoped>
.table-page-search-wrapper {
  margin-bottom: 16px;
}

.alert-stats {
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
    
    &.critical {
      .stat-icon {
        background: linear-gradient(135deg, #ff4d4f 0%, #ff7875 100%);
      }
      .stat-value {
        color: #ff4d4f;
      }
    }
    
    &.high {
      .stat-icon {
        background: linear-gradient(135deg, #fa8c16 0%, #ffa940 100%);
      }
      .stat-value {
        color: #fa8c16;
      }
    }
    
    &.medium {
      .stat-icon {
        background: linear-gradient(135deg, #faad14 0%, #ffc53d 100%);
      }
      .stat-value {
        color: #faad14;
      }
    }
    
    &.pending {
      .stat-icon {
        background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);
      }
      .stat-value {
        color: #722ed1;
      }
    }
  }
}

.alert-message,
.event-details {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 400px;
}
</style>
