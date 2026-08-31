<template>
  <page-layout>
    <div slot="headerContent">
      <div class="header-info">
        <div class="welcome-text">
          <h1>Artemis HIDS 安全监控中心</h1>
          <p class="subtitle">实时监控主机安全状态，快速响应安全威胁</p>
        </div>
      </div>
    </div>
    
    <div class="dashboard-content">
      <!-- 未认证提示 -->
      <a-alert
        v-if="!isAuthenticated"
        message="提示：您尚未登录"
        description="仪表盘数据需要登录后才能查看。请点击右上角登录按钮进行登录。"
        type="warning"
        show-icon
        closable
        style="margin-bottom: 24px;"
      />
      
      <!-- 关键指标卡片 -->
      <a-row :gutter="24" class="metrics-row">
        <a-col :xs="24" :sm="12" :lg="8">
          <a-card :bordered="false" class="metric-card">
            <div class="metric-content">
              <div class="metric-icon" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);">
                <a-icon type="desktop" />
              </div>
              <div class="metric-info">
                <div class="metric-value">{{ metrics.onlineRate.toFixed(1) }}%</div>
                <div class="metric-label">主机在线率</div>
                <div class="metric-detail">{{ metrics.onlineAgents }}/{{ metrics.totalAgents }} 在线</div>
              </div>
            </div>
          </a-card>
        </a-col>
        
        <a-col :xs="24" :sm="12" :lg="8">
          <a-card :bordered="false" class="metric-card">
            <div class="metric-content">
              <div class="metric-icon" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);">
                <a-icon type="warning" />
              </div>
              <div class="metric-info">
                <div class="metric-value alert">{{ metrics.todayAlerts }}</div>
                <div class="metric-label">今日告警条数</div>
                <div class="metric-detail">告警总条数 {{ metrics.totalAlerts }}</div>
              </div>
            </div>
          </a-card>
        </a-col>

        <a-col :xs="24" :sm="12" :lg="8">
          <a-card :bordered="false" class="metric-card">
            <div class="metric-content">
              <div class="metric-icon" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);">
                <a-icon type="clock-circle" />
              </div>
              <div class="metric-info">
                <div class="metric-value pending">{{ metrics.pendingAlerts }}</div>
                <div class="metric-label">待处理告警</div>
                <div class="metric-detail">需要立即处理</div>
              </div>
            </div>
          </a-card>
        </a-col>
      </a-row>
      
      <!-- 主要内容区 -->
      <a-row :gutter="24" style="margin-top: 24px;">
        <a-col :xs="24" :lg="16">
          <!-- 最近告警 -->
          <a-card title="最近告警" :bordered="false" class="content-card">
            <a-table 
              :columns="alertColumns" 
              :data-source="recentAlerts" 
              :pagination="false"
              size="small"
            >
              <template slot="level" slot-scope="level">
                <a-tag :color="getLevelColor(level)">
                  {{ getLevelText(level) }}
                </a-tag>
              </template>
              <template slot="time" slot-scope="time">
                {{ formatTime(time) }}
              </template>
            </a-table>
          </a-card>
          
          <!-- 主机状态 -->
          <a-card title="主机状态" :bordered="false" class="content-card" style="margin-top: 24px;">
            <a-table 
              :columns="hostColumns" 
              :data-source="hosts" 
              :pagination="false"
              size="small"
            >
              <template slot="status" slot-scope="status">
                <a-badge :status="status === 'online' ? 'success' : 'default'" :text="status === 'online' ? '在线' : '离线'" />
              </template>
              <template slot="cpu" slot-scope="cpu">
                <a-progress :percent="cpu" size="small" :status="cpu > 80 ? 'exception' : 'normal'" />
              </template>
              <template slot="memory" slot-scope="memory">
                <a-progress :percent="memory" size="small" :status="memory > 80 ? 'exception' : 'normal'" />
              </template>
            </a-table>
          </a-card>
        </a-col>
        
        <a-col :xs="24" :lg="8">
          <!-- 威胁类型分布 -->
          <a-card title="威胁类型分布" :bordered="false" class="content-card">
            <div class="threat-list">
              <div v-for="(item, index) in threatTypes" :key="index" class="threat-item">
                <div class="threat-info">
                  <span class="threat-name">{{ item.name }}</span>
                  <span class="threat-count">{{ item.count }}</span>
                </div>
                <a-progress :percent="item.percent" :show-info="false" :stroke-color="item.color" />
              </div>
            </div>
          </a-card>
        </a-col>
      </a-row>
    </div>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'

export default {
  name: 'ArtemisDashboard',
  components: { PageLayout },
  data() {
    return {
      isAuthenticated: false, // 是否已认证
      metrics: {
        totalAgents: 0,
        onlineAgents: 0,
        onlineRate: 0,
        todayAlerts: 0,
        pendingAlerts: 0,
        totalAlerts: 0
      },
      alertColumns: [
        {
          title: '级别',
          dataIndex: 'level',
          key: 'level',
          width: 80,
          scopedSlots: { customRender: 'level' }
        },
        {
          title: '告警内容',
          dataIndex: 'message',
          key: 'message',
          ellipsis: true
        },
        {
          title: '主机',
          dataIndex: 'host',
          key: 'host',
          width: 120
        },
        {
          title: '时间',
          dataIndex: 'time',
          key: 'time',
          width: 100,
          scopedSlots: { customRender: 'time' }
        }
      ],
      recentAlerts: [
        { level: 'critical', message: '检测到异常登录行为', host: 'web-server-01', time: new Date() },
        { level: 'warning', message: '文件完整性校验失败', host: 'db-server-02', time: new Date(Date.now() - 300000) },
        { level: 'info', message: '发现新的网络连接', host: 'app-server-03', time: new Date(Date.now() - 600000) },
        { level: 'warning', message: '进程异常启动', host: 'web-server-02', time: new Date(Date.now() - 900000) },
        { level: 'critical', message: '可疑的系统调用', host: 'api-server-01', time: new Date(Date.now() - 1200000) }
      ],
      hostColumns: [
        {
          title: '主机名',
          dataIndex: 'hostname',
          key: 'hostname'
        },
        {
          title: '状态',
          dataIndex: 'status',
          key: 'status',
          width: 80,
          scopedSlots: { customRender: 'status' }
        },
        {
          title: 'CPU',
          dataIndex: 'cpu',
          key: 'cpu',
          width: 120,
          scopedSlots: { customRender: 'cpu' }
        },
        {
          title: '内存',
          dataIndex: 'memory',
          key: 'memory',
          width: 120,
          scopedSlots: { customRender: 'memory' }
        }
      ],
      hosts: [
        { hostname: 'web-server-01', status: 'online', cpu: 45, memory: 62 },
        { hostname: 'db-server-02', status: 'online', cpu: 78, memory: 85 },
        { hostname: 'app-server-03', status: 'online', cpu: 32, memory: 48 },
        { hostname: 'api-server-01', status: 'online', cpu: 56, memory: 71 }
      ],
      threatTypes: [
        { name: '异常登录', count: 45, percent: 35, color: '#f5222d' },
        { name: '文件篡改', count: 32, percent: 25, color: '#fa8c16' },
        { name: '进程异常', count: 28, percent: 22, color: '#faad14' },
        { name: '网络异常', count: 23, percent: 18, color: '#1890ff' }
      ]
    }
  },
  methods: {
    async fetchDashboardStats() {
      try {
        const token = localStorage.getItem('token')
        
        // 如果没有token，跳过请求
        if (!token || token === 'null' || token === 'undefined') {
          console.warn('未找到有效的认证token，显示默认数据')
          this.isAuthenticated = false
          return
        }
        
        const response = await fetch('/api/v1/stats/dashboard', {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        })
        
        if (response.ok) {
          const data = await response.json()
          this.metrics = {
            totalAgents: data.total_agents || 0,
            onlineAgents: data.online_agents || 0,
            onlineRate: data.online_rate || 0,
            todayAlerts: data.today_alerts || 0,
            pendingAlerts: data.pending_alerts || 0,
            totalAlerts: data.total_alerts || 0
          }
          this.isAuthenticated = true
        } else if (response.status === 401) {
          console.warn('认证失败，token可能已过期，请重新登录')
          this.isAuthenticated = false
          // 可以考虑跳转到登录页
          // this.$router.push('/login')
        } else {
          console.error('获取仪表盘数据失败:', response.status, response.statusText)
          this.isAuthenticated = false
        }
      } catch (error) {
        console.error('获取仪表盘统计数据失败:', error)
        this.isAuthenticated = false
      }
    },
    getLevelColor(level) {
      const colors = {
        critical: 'red',
        warning: 'orange',
        info: 'blue'
      }
      return colors[level] || 'default'
    },
    getLevelText(level) {
      const texts = {
        critical: '严重',
        warning: '警告',
        info: '信息'
      }
      return texts[level] || level
    },
    formatTime(time) {
      const now = new Date()
      const diff = Math.floor((now - time) / 1000)
      
      if (diff < 60) return '刚刚'
      if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
      if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
      return `${Math.floor(diff / 86400)}天前`
    },
    checkAuthStatus() {
      // 检查认证状态
      const token = localStorage.getItem('token')
      this.isAuthenticated = !!(token && token !== 'null' && token !== 'undefined')
      return this.isAuthenticated
    }
  },
  watch: {
    // 监听路由变化，从登录页返回时重新检查认证状态
    '$route'() {
      this.checkAuthStatus()
      if (this.isAuthenticated) {
        this.fetchDashboardStats()
      }
    }
  },
  mounted() {
    // 检查并设置认证状态
    this.checkAuthStatus()
    
    // 加载仪表盘数据
    this.fetchDashboardStats()
    
    // 每30秒刷新一次数据
    this.statsTimer = setInterval(() => {
      // 每次定时刷新前先检查认证状态
      if (this.checkAuthStatus()) {
        this.fetchDashboardStats()
      }
    }, 30000)
    
    console.log('Artemis Dashboard mounted')
  },
  beforeDestroy() {
    // 清理定时器
    if (this.statsTimer) {
      clearInterval(this.statsTimer)
    }
  }
}
</script>

<style lang="less" scoped>
.header-info {
  .welcome-text {
    h1 {
      font-size: 28px;
      font-weight: 600;
      color: #262626;
      margin-bottom: 8px;
    }
    
    .subtitle {
      font-size: 14px;
      color: #8c8c8c;
      margin: 0;
    }
  }
}

.dashboard-content {
  padding: 24px;
  background: #f0f2f5;
  min-height: calc(100vh - 200px);
}

.metrics-row {
  margin-bottom: 24px;
}

.metric-card {
  border-radius: 8px;
  overflow: hidden;
  transition: all 0.3s ease;
  
  &:hover {
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
    transform: translateY(-4px);
  }
  
  /deep/ .ant-card-body {
    padding: 20px;
  }
}

.metric-content {
  display: flex;
  align-items: center;
  
  .metric-icon {
    width: 64px;
    height: 64px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-right: 16px;
    
    .anticon {
      font-size: 32px;
      color: #fff;
    }
  }
  
  .metric-info {
    flex: 1;
    
    .metric-value {
      font-size: 28px;
      font-weight: 600;
      color: #262626;
      line-height: 1.2;
      
      &.alert {
        color: #f5222d;
      }
      
      &.pending {
        color: #fa8c16;
      }
    }
    
    .metric-label {
      font-size: 14px;
      color: #8c8c8c;
      margin-top: 4px;
    }
    
    .metric-detail {
      font-size: 12px;
      color: #bfbfbf;
      margin-top: 2px;
    }
  }
}

.content-card {
  border-radius: 8px;
  
  /deep/ .ant-card-head {
    border-bottom: 1px solid #f0f0f0;
    
    .ant-card-head-title {
      font-size: 16px;
      font-weight: 600;
    }
  }
}

.threat-list {
  .threat-item {
    margin-bottom: 20px;
    
    &:last-child {
      margin-bottom: 0;
    }
    
    .threat-info {
      display: flex;
      justify-content: space-between;
      margin-bottom: 8px;
      
      .threat-name {
        font-size: 14px;
        color: #262626;
      }
      
      .threat-count {
        font-size: 14px;
        font-weight: 600;
        color: #262626;
      }
    }
  }
}

.quick-actions {
  /deep/ .ant-btn {
    height: 40px;
    font-size: 14px;
  }
}

.system-info {
  .info-item {
    display: flex;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;
    
    &:last-child {
      border-bottom: none;
    }
    
    .info-label {
      font-size: 14px;
      color: #8c8c8c;
    }
    
    .info-value {
      font-size: 14px;
      color: #262626;
      font-weight: 500;
    }
  }
}

@media screen and (max-width: 768px) {
  .dashboard-content {
    padding: 12px;
  }
  
  .metric-content {
    .metric-icon {
      width: 48px;
      height: 48px;
      
      .anticon {
        font-size: 24px;
      }
    }
    
    .metric-info {
      .metric-value {
        font-size: 22px;
      }
    }
  }
}
</style>
