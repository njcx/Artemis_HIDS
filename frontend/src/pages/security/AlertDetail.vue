<template>
  <page-layout>
    <a-card :bordered="false">
      <!-- 返回按钮和标题 -->
      <div class="detail-header">
        <a-button icon="arrow-left" @click="$router.go(-1)">返回</a-button>
        <h2>告警详情</h2>
      </div>
      
      <a-spin :spinning="loading">
        <!-- 告警概览卡片 -->
        <div class="alert-overview">
          <a-row :gutter="16">
            <a-col :span="6">
              <div class="stat-card" :class="getSeverityClass(alert.severity)">
                <div class="stat-icon">
                  <a-icon :type="getSeverityIcon(alert.severity)" />
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ getSeverityText(alert.severity) }}</div>
                  <div class="stat-label">告警级别</div>
                </div>
              </div>
            </a-col>
            <a-col :span="6">
              <div class="stat-card status">
                <div class="stat-icon" :style="getStatusIconStyle(alert.status)">
                  <a-icon :type="getStatusIcon(alert.status)" />
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ getStatusText(alert.status) }}</div>
                  <div class="stat-label">处理状态</div>
                </div>
              </div>
            </a-col>
            <a-col :span="6">
              <div class="stat-card host">
                <div class="stat-icon">
                  <a-icon type="desktop" />
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ alert.hostname || 'N/A' }}</div>
                  <div class="stat-label">主机名称</div>
                </div>
              </div>
            </a-col>
            <a-col :span="6">
              <div class="stat-card time">
                <div class="stat-icon">
                  <a-icon type="clock-circle" />
                </div>
                <div class="stat-content">
                  <div class="stat-value time-value">{{ formatTimeShort(alert.timestamp) }}</div>
                  <div class="stat-label">告警时间</div>
                </div>
              </div>
            </a-col>
          </a-row>
        </div>

        <!-- 基本信息 -->
        <a-card title="基本信息" class="info-card" :bordered="false">
          <template slot="extra">
            <a-tag :color="getSeverityColor(alert.severity)" style="margin-right: 0">
              <a-icon :type="getSeverityIcon(alert.severity)" />
              {{ getSeverityText(alert.severity) }}
            </a-tag>
          </template>
          <a-row :gutter="16">
            <a-col :span="12">
              <div class="info-item">
                <span class="info-label">告警ID：</span>
                <span class="info-value">{{ alert.id || 'N/A' }}</span>
              </div>
            </a-col>
            <a-col :span="12">
              <div class="info-item">
                <span class="info-label">主机ID：</span>
                <span class="info-value">{{ alert.agent_id || 'N/A' }}</span>
              </div>
            </a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 16px;">
            <a-col :span="12">
              <div class="info-item">
                <span class="info-label">事件类型：</span>
                <a-tag color="blue">{{ alert.event_type || 'N/A' }}</a-tag>
              </div>
            </a-col>
            <a-col :span="12">
              <div class="info-item">
                <span class="info-label">告警状态：</span>
                <a-badge :status="getStatusBadge(alert.status)" :text="getStatusText(alert.status)" />
              </div>
            </a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 16px;">
            <a-col :span="24">
              <div class="info-item">
                <span class="info-label">告警标题：</span>
                <span class="info-value title-value">{{ alert.title || 'N/A' }}</span>
              </div>
            </a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 16px;">
            <a-col :span="24">
              <div class="info-item">
                <span class="info-label">详细描述：</span>
                <span class="info-value">{{ alert.description || '暂无详细描述' }}</span>
              </div>
            </a-col>
          </a-row>
        </a-card>

        <!-- 告警详细信息 -->
        <a-card title="告警详细信息" class="info-card" :bordered="false" style="margin-top: 16px;">
          <template slot="extra">
            <a-icon type="file-search" style="color: #1890ff" />
          </template>
          <a-row :gutter="16" v-if="alert.process_name">
            <a-col :span="12">
              <div class="info-item">
                <span class="info-label">进程名称：</span>
                <a-tag color="cyan">{{ alert.process_name }}</a-tag>
              </div>
            </a-col>
            <a-col :span="12" v-if="alert.user">
              <div class="info-item">
                <span class="info-label">执行用户：</span>
                <a-tag color="purple">{{ alert.user }}</a-tag>
              </div>
            </a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 16px;" v-if="alert.process_path">
            <a-col :span="24">
              <div class="info-item">
                <span class="info-label">进程路径：</span>
                <code class="code-value">{{ alert.process_path }}</code>
              </div>
            </a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 16px;" v-if="alert.details">
            <a-col :span="24">
              <div class="info-item">
                <span class="info-label">详细信息：</span>
                <pre class="details-pre">{{ formatDetails(alert.details) }}</pre>
              </div>
            </a-col>
          </a-row>
          <a-empty v-if="!alert.process_name && !alert.process_path && !alert.user && !alert.details" description="暂无详细信息" />
        </a-card>

        <!-- 处理记录 -->
        <a-card title="处理记录" class="info-card" :bordered="false" style="margin-top: 16px;">
          <template slot="extra">
            <a-icon type="history" style="color: #722ed1" />
          </template>
          <a-timeline class="timeline-custom">
            <a-timeline-item color="blue">
              <div class="timeline-content">
                <div class="timeline-title">
                  <a-icon type="bell" style="margin-right: 8px" />
                  告警创建
                </div>
                <div class="timeline-time">{{ formatTime(alert.created_at) }}</div>
              </div>
            </a-timeline-item>
            <a-timeline-item v-if="alert.status === 'investigating'" color="orange">
              <div class="timeline-content">
                <div class="timeline-title">
                  <a-icon type="eye" style="margin-right: 8px" />
                  告警调查中
                </div>
                <div class="timeline-time">{{ formatTime(alert.updated_at) }}</div>
              </div>
            </a-timeline-item>
            <a-timeline-item v-if="alert.status === 'resolved'" color="green">
              <div class="timeline-content">
                <div class="timeline-title">
                  <a-icon type="check-circle" style="margin-right: 8px" />
                  告警已解决
                </div>
                <div class="timeline-time">{{ formatTime(alert.updated_at) }}</div>
              </div>
            </a-timeline-item>
            <a-timeline-item v-if="alert.status === 'ignored'" color="gray">
              <div class="timeline-content">
                <div class="timeline-title">
                  <a-icon type="stop" style="margin-right: 8px" />
                  告警已忽略
                </div>
                <div class="timeline-time">{{ formatTime(alert.updated_at) }}</div>
              </div>
            </a-timeline-item>
          </a-timeline>
        </a-card>

        <!-- 操作按钮 -->
        <a-card class="action-card" :bordered="false" style="margin-top: 16px;">
          <div class="action-buttons" v-if="alert.status === 'open' || alert.status === 'investigating'">
            <a-button type="default" size="large" @click="handleProcess('investigating')" v-if="alert.status === 'open'">
              <a-icon type="eye" /> 标记为调查中
            </a-button>
            <a-button type="primary" size="large" @click="handleProcess('resolved')">
              <a-icon type="check-circle" /> 标记为已解决
            </a-button>
            <a-button size="large" @click="handleProcess('ignored')">
              <a-icon type="close-circle" /> 忽略告警
            </a-button>
          </div>
          <div class="action-result" v-else>
            <a-result
              :status="alert.status === 'resolved' ? 'success' : 'info'"
              :title="`此告警${getStatusText(alert.status)}`"
              :sub-title="formatTime(alert.updated_at)"
            />
          </div>
        </a-card>
      </a-spin>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import { getAlertDetail, updateAlertStatus } from '@/services/alert'

export default {
  name: 'AlertDetail',
  components: {
    PageLayout
  },
  data() {
    return {
      loading: false,
      alert: {
        id: '',
        severity: '',
        title: '',
        description: '',
        agent_id: '',
        hostname: '',
        status: '',
        timestamp: null,
        created_at: null,
        updated_at: null,
        event_type: '',
        source: '',
        process_name: '',
        process_path: '',
        user: '',
        details: null
      }
    }
  },
  mounted() {
    this.loadAlertDetail()
  },
  methods: {
    async loadAlertDetail() {
      const alertId = this.$route.params.id || this.$route.query.alertId
      if (!alertId) {
        this.$message.error('告警ID不存在')
        this.$router.go(-1)
        return
      }
      
      this.loading = true
      try {
        const data = await getAlertDetail(alertId)
        this.alert = data || {}
      } catch (error) {
        this.$message.error('获取告警详情失败')
        console.error(error)
      } finally {
        this.loading = false
      }
    },
    async handleProcess(status) {
      const actionMap = {
        investigating: '标记为调查中',
        resolved: '标记为已解决',
        ignored: '忽略'
      }
      const actionText = actionMap[status] || '处理'
      this.$confirm({
        title: '确认操作',
        content: `确定要${actionText}此告警吗？`,
        onOk: async () => {
          try {
            await updateAlertStatus(this.alert.id, { status })
            this.$message.success(`${actionText}成功`)
            // 重新加载详情
            await this.loadAlertDetail()
          } catch (error) {
            this.$message.error(`${actionText}失败`)
            console.error(error)
          }
        }
      })
    },
    getSeverityClass(severity) {
      const classes = {
        critical: 'critical',
        high: 'high',
        medium: 'medium',
        low: 'low'
      }
      return classes[severity] || 'low'
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
      return texts[severity] || severity || 'N/A'
    },
    getStatusBadge(status) {
      const badges = {
        open: 'error',
        investigating: 'processing',
        resolved: 'success',
        ignored: 'default'
      }
      return badges[status] || 'default'
    },
    getStatusText(status) {
      const texts = {
        open: '待处理',
        investigating: '调查中',
        resolved: '已解决',
        ignored: '已忽略'
      }
      return texts[status] || status || 'N/A'
    },
    getStatusIcon(status) {
      const icons = {
        open: 'exclamation-circle',
        investigating: 'eye',
        resolved: 'check-circle',
        ignored: 'stop'
      }
      return icons[status] || 'question-circle'
    },
    getStatusIconStyle(status) {
      const styles = {
        open: 'background: linear-gradient(135deg, #ff4d4f 0%, #ff7875 100%);',
        investigating: 'background: linear-gradient(135deg, #fa8c16 0%, #ffa940 100%);',
        resolved: 'background: linear-gradient(135deg, #52c41a 0%, #73d13d 100%);',
        ignored: 'background: linear-gradient(135deg, #bfbfbf 0%, #d9d9d9 100%);'
      }
      return styles[status] || 'background: linear-gradient(135deg, #1890ff 0%, #40a9ff 100%);'
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
    },
    formatTimeShort(time) {
      if (!time) return 'N/A'
      const date = new Date(time)
      return `${date.getMonth() + 1}/${date.getDate()} ${date.getHours()}:${String(date.getMinutes()).padStart(2, '0')}`
    },
    formatDetails(details) {
      if (!details) return 'N/A'
      if (typeof details === 'string') {
        try {
          return JSON.stringify(JSON.parse(details), null, 2)
        } catch {
          return details
        }
      }
      return JSON.stringify(details, null, 2)
    }
  }
}
</script>

<style lang="less" scoped>
.detail-header {
  display: flex;
  align-items: center;
  margin-bottom: 24px;
  
  h2 {
    margin: 0 0 0 16px;
    font-size: 20px;
    font-weight: 600;
  }
}

.alert-overview {
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
        font-size: 20px;
        font-weight: 600;
        line-height: 1.2;
        margin-bottom: 4px;
        
        &.time-value {
          font-size: 16px;
        }
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
    
    &.low {
      .stat-icon {
        background: linear-gradient(135deg, #1890ff 0%, #40a9ff 100%);
      }
      .stat-value {
        color: #1890ff;
      }
    }
    
    &.host {
      .stat-icon {
        background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);
      }
      .stat-value {
        color: #722ed1;
        font-size: 16px;
        word-break: break-all;
      }
    }
    
    &.time {
      .stat-icon {
        background: linear-gradient(135deg, #13c2c2 0%, #36cfc9 100%);
      }
      .stat-value {
        color: #13c2c2;
      }
    }
  }
}

.info-card {
  /deep/ .ant-card-head {
    border-bottom: 1px solid #f0f0f0;
    
    .ant-card-head-title {
      font-weight: 600;
    }
  }
  
  /deep/ .ant-card-body {
    padding: 24px;
  }
}

.info-item {
  display: flex;
  align-items: flex-start;
  line-height: 32px;
  
  .info-label {
    font-weight: 500;
    color: #595959;
    margin-right: 8px;
    min-width: 80px;
    flex-shrink: 0;
  }
  
  .info-value {
    color: #262626;
    font-weight: 400;
    word-break: break-all;
    
    &.title-value {
      font-weight: 500;
      font-size: 15px;
    }
  }
  
  .code-value {
    background: #f5f5f5;
    padding: 4px 8px;
    border-radius: 4px;
    font-family: 'Monaco', 'Menlo', monospace;
    font-size: 13px;
    color: #d4380d;
    word-break: break-all;
  }
}

.details-pre {
  margin: 8px 0 0 0;
  padding: 16px;
  background: #f5f5f5;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.timeline-custom {
  margin-top: 8px;
  padding-left: 8px;
  
  .timeline-content {
    .timeline-title {
      font-weight: 500;
      font-size: 15px;
      color: #262626;
      margin-bottom: 4px;
    }
    
    .timeline-time {
      font-size: 13px;
      color: #8c8c8c;
    }
  }
}

.action-card {
  /deep/ .ant-card-body {
    padding: 24px;
  }
}

.action-buttons {
  display: flex;
  justify-content: center;
  gap: 16px;
  
  .ant-btn {
    min-width: 140px;
  }
}

.action-result {
  /deep/ .ant-result {
    padding: 24px 0;
  }
}
</style>
