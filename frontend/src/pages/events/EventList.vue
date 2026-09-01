<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="6" :sm="24">
              <a-form-item label="事件类型">
                <a-select v-model="queryParams.type" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="file">文件监控</a-select-option>
                  <a-select-option value="process">进程监控</a-select-option>
                  <a-select-option value="network">网络监控</a-select-option>
                  <a-select-option value="login">登录审计</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-form-item label="主机">
                <a-input v-model="queryParams.host" placeholder="请输入主机名" />
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-form-item label="时间范围">
                <a-range-picker v-model="queryParams.dateRange" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-button type="primary" @click="handleSearch">查询</a-button>
              <a-button style="margin-left: 8px" @click="handleReset">重置</a-button>
              <a-button type="link" icon="export" @click="handleExport">导出</a-button>
            </a-col>
          </a-row>
        </a-form>
      </div>

      <a-table
        :columns="columns"
        :data-source="data"
        :pagination="pagination"
        :loading="loading"
        @change="handleTableChange"
        :scroll="{ x: 1200 }"
        rowKey="id"
      >
        <template slot="type" slot-scope="type">
          <a-tag :color="getTypeColor(type)">
            <a-icon :type="getTypeIcon(type)" />
            {{ getTypeText(type) }}
          </a-tag>
        </template>
        
        <template slot="details" slot-scope="details">
          <div class="event-details">{{ details }}</div>
        </template>
        
        <template slot="time" slot-scope="time">
          {{ new Date(time).toLocaleString() }}
        </template>
        
        <template slot="action" slot-scope="text, record">
          <a @click="handleView(record)">查看详情</a>
        </template>
      </a-table>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'

export default {
  name: 'EventList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        type: '',
        host: '',
        dateRange: []
      },
      loading: false,
      data: [
        {
          id: 1,
          type: 'file',
          host: 'web-server-01',
          details: '文件被修改: /etc/nginx/nginx.conf',
          user: 'root',
          time: new Date()
        },
        {
          id: 2,
          type: 'process',
          host: 'db-server-02',
          details: '新进程启动: /usr/bin/mysqld',
          user: 'mysql',
          time: new Date(Date.now() - 300000)
        },
        {
          id: 3,
          type: 'network',
          host: 'app-server-03',
          details: '建立网络连接: 192.168.1.103:8080 -> 45.123.45.67:443',
          user: 'www-data',
          time: new Date(Date.now() - 600000)
        },
        {
          id: 4,
          type: 'login',
          host: 'api-server-01',
          details: 'SSH登录成功: 来自 192.168.1.200',
          user: 'admin',
          time: new Date(Date.now() - 900000)
        },
        {
          id: 5,
          type: 'file',
          host: 'web-server-02',
          details: '文件被删除: /var/log/access.log',
          user: 'root',
          time: new Date(Date.now() - 1200000)
        },
        {
          id: 6,
          type: 'process',
          host: 'web-server-01',
          details: '进程异常退出: /usr/sbin/apache2 (exit code: 1)',
          user: 'www-data',
          time: new Date(Date.now() - 1500000)
        }
      ],
      columns: [
        {
          title: '事件类型',
          dataIndex: 'type',
          key: 'type',
          width: 120,
          scopedSlots: { customRender: 'type' }
        },
        {
          title: '主机',
          dataIndex: 'host',
          key: 'host',
          width: 150
        },
        {
          title: '事件详情',
          dataIndex: 'details',
          key: 'details',
          scopedSlots: { customRender: 'details' }
        },
        {
          title: '用户',
          dataIndex: 'user',
          key: 'user',
          width: 100
        },
        {
          title: '时间',
          dataIndex: 'time',
          key: 'time',
          width: 180,
          scopedSlots: { customRender: 'time' }
        },
        {
          title: '操作',
          key: 'action',
          width: 120,
          fixed: 'right',
          scopedSlots: { customRender: 'action' }
        }
      ],
      pagination: {
        current: 1,
        pageSize: 10,
        total: 6,
        showSizeChanger: true,
        showQuickJumper: true,
        showTotal: total => `共 ${total} 条记录`
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
        type: '',
        host: '',
        dateRange: []
      }
    },
    handleExport() {
      this.$message.info('导出功能开发中')
    },
    handleView(record) {
      this.$message.info(`查看事件详情: ${record.id}`)
    },
    handleTableChange(pagination) {
      this.pagination = pagination
      this.handleSearch()
    },
    getTypeColor(type) {
      const colors = {
        file: 'blue',
        process: 'green',
        network: 'orange',
        login: 'purple'
      }
      return colors[type] || 'default'
    },
    getTypeIcon(type) {
      const icons = {
        file: 'file',
        process: 'code',
        network: 'global',
        login: 'user'
      }
      return icons[type] || 'file'
    },
    getTypeText(type) {
      const texts = {
        file: '文件监控',
        process: '进程监控',
        network: '网络监控',
        login: '登录审计'
      }
      return texts[type] || type
    }
  }
}
</script>

<style lang="less" scoped>
.table-page-search-wrapper {
  margin-bottom: 16px;
}

.event-details {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 500px;
}
</style>
