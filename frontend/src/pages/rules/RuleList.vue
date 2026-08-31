<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="8" :sm="24">
              <a-form-item label="规则名称">
                <a-input v-model="queryParams.name" placeholder="请输入规则名称" />
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-form-item label="规则类型">
                <a-select v-model="queryParams.category" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="file">文件监控</a-select-option>
                  <a-select-option value="process">进程监控</a-select-option>
                  <a-select-option value="network">网络监控</a-select-option>
                  <a-select-option value="system">系统监控</a-select-option>
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
        <a-button type="primary" icon="plus" @click="handleAdd">新增规则</a-button>
        <a-button icon="import" @click="handleImport">导入规则</a-button>
        <a-button icon="export" @click="handleExport">导出规则</a-button>
      </div>

      <a-table
        :columns="columns"
        :data-source="data"
        :pagination="pagination"
        :loading="loading"
        @change="handleTableChange"
        rowKey="id"
      >
        <template slot="category" slot-scope="category">
          <a-tag :color="getCategoryColor(category)">
            {{ getCategoryText(category) }}
          </a-tag>
        </template>
        
        <template slot="severity" slot-scope="severity">
          <a-tag :color="getSeverityColor(severity)">
            {{ getSeverityText(severity) }}
          </a-tag>
        </template>
        
        <template slot="enabled" slot-scope="enabled">
          <a-switch :checked="enabled" @change="(checked) => handleToggle(enabled, checked)" />
        </template>
        
        <template slot="action" slot-scope="text, record">
          <a @click="handleEdit(record)">编辑</a>
          <a-divider type="vertical" />
          <a @click="handleTest(record)">测试</a>
          <a-divider type="vertical" />
          <a-popconfirm title="确定要删除这条规则吗？" @confirm="handleDelete(record)">
            <a style="color: #f5222d">删除</a>
          </a-popconfirm>
        </template>
      </a-table>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'

export default {
  name: 'RuleList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        name: '',
        category: ''
      },
      loading: false,
      data: [
        {
          id: 1,
          name: '检测异常登录',
          category: 'system',
          severity: 'critical',
          description: '检测来自异常IP的登录尝试',
          enabled: true,
          hitCount: 156
        },
        {
          id: 2,
          name: '监控敏感文件修改',
          category: 'file',
          severity: 'critical',
          description: '监控 /etc/passwd, /etc/shadow 等敏感文件的修改',
          enabled: true,
          hitCount: 23
        },
        {
          id: 3,
          name: '异常进程启动',
          category: 'process',
          severity: 'warning',
          description: '检测从 /tmp 目录启动的可执行文件',
          enabled: true,
          hitCount: 78
        },
        {
          id: 4,
          name: '外联检测',
          category: 'network',
          severity: 'warning',
          description: '监控到外部IP的异常连接',
          enabled: false,
          hitCount: 234
        },
        {
          id: 5,
          name: 'SSH暴力破解',
          category: 'system',
          severity: 'critical',
          description: '检测短时间内多次SSH登录失败',
          enabled: true,
          hitCount: 45
        }
      ],
      columns: [
        {
          title: '规则名称',
          dataIndex: 'name',
          key: 'name',
          width: 180
        },
        {
          title: '规则类型',
          dataIndex: 'category',
          key: 'category',
          width: 120,
          scopedSlots: { customRender: 'category' }
        },
        {
          title: '严重程度',
          dataIndex: 'severity',
          key: 'severity',
          width: 100,
          scopedSlots: { customRender: 'severity' }
        },
        {
          title: '规则描述',
          dataIndex: 'description',
          key: 'description'
        },
        {
          title: '命中次数',
          dataIndex: 'hitCount',
          key: 'hitCount',
          width: 100
        },
        {
          title: '启用状态',
          dataIndex: 'enabled',
          key: 'enabled',
          width: 100,
          scopedSlots: { customRender: 'enabled' }
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
        total: 5,
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
        name: '',
        category: ''
      }
    },
    handleAdd() {
      this.$message.info('新增规则功能')
    },
    handleImport() {
      this.$message.info('导入规则功能')
    },
    handleExport() {
      this.$message.info('导出规则功能')
    },
    handleEdit(record) {
      this.$message.info(`编辑规则: ${record.name}`)
    },
    handleTest(record) {
      this.$message.info(`测试规则: ${record.name}`)
    },
    handleDelete(record) {
      this.$message.success(`已删除规则: ${record.name}`)
    },
    handleToggle(record, checked) {
      this.$message.success(`已${checked ? '启用' : '禁用'}规则`)
    },
    handleTableChange(pagination) {
      this.pagination = pagination
      this.handleSearch()
    },
    getCategoryColor(category) {
      const colors = {
        file: 'blue',
        process: 'green',
        network: 'orange',
        system: 'purple'
      }
      return colors[category] || 'default'
    },
    getCategoryText(category) {
      const texts = {
        file: '文件监控',
        process: '进程监控',
        network: '网络监控',
        system: '系统监控'
      }
      return texts[category] || category
    },
    getSeverityColor(severity) {
      const colors = {
        critical: 'red',
        warning: 'orange',
        info: 'blue'
      }
      return colors[severity] || 'default'
    },
    getSeverityText(severity) {
      const texts = {
        critical: '严重',
        warning: '警告',
        info: '信息'
      }
      return texts[severity] || severity
    }
  }
}
</script>

<style lang="less" scoped>
.table-page-search-wrapper {
  margin-bottom: 16px;
}

.table-operator {
  margin-bottom: 16px;
  
  button {
    margin-right: 8px;
  }
}
</style>
