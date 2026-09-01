<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="6" :sm="24">
              <a-form-item label="规则名称">
                <a-input v-model="queryParams.name" placeholder="请输入规则名称" />
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-form-item label="规则类型">
                <a-select v-model="queryParams.type" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="pattern">模式匹配</a-select-option>
                  <a-select-option value="threshold">阈值检测</a-select-option>
                  <a-select-option value="correlation">关联分析</a-select-option>
                  <a-select-option value="statistical">统计分析</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-form-item label="严重程度">
                <a-select v-model="queryParams.severity" placeholder="请选择" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option value="critical">严重</a-select-option>
                  <a-select-option value="high">高</a-select-option>
                  <a-select-option value="medium">中</a-select-option>
                  <a-select-option value="low">低</a-select-option>
                  <a-select-option value="info">信息</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :md="6" :sm="24">
              <a-button type="primary" @click="handleSearch">查询</a-button>
              <a-button style="margin-left: 8px" @click="handleReset">重置</a-button>
            </a-col>
          </a-row>
        </a-form>
      </div>

      <div class="table-operator">
        <a-button type="primary" icon="plus" @click="handleAdd">新增规则</a-button>
      </div>

      <a-table
        :columns="columns"
        :data-source="data"
        :pagination="pagination"
        :loading="loading"
        @change="handleTableChange"
        rowKey="id"
      >
        <template slot="type" slot-scope="type">
          <a-tag :color="getTypeColor(type)">
            {{ getTypeText(type) }}
          </a-tag>
        </template>

        <template slot="severity" slot-scope="severity">
          <a-tag :color="getSeverityColor(severity)">
            {{ getSeverityText(severity) }}
          </a-tag>
        </template>

        <template slot="enabled" slot-scope="enabled, record">
          <a-switch :checked="enabled" @change="checked => handleToggle(record, checked)" />
        </template>

        <template slot="action" slot-scope="text, record">
          <a @click="handleEdit(record)">编辑</a>
          <a-divider type="vertical" />
          <a-popconfirm title="确定要删除这条规则吗？此操作不可恢复！" @confirm="handleDelete(record)">
            <a style="color: #f5222d">删除</a>
          </a-popconfirm>
        </template>
      </a-table>
    </a-card>

    <!-- 新增/编辑规则对话框 -->
    <a-modal
      :title="modalTitle"
      :visible="visible"
      :confirm-loading="confirmLoading"
      @ok="handleOk"
      @cancel="handleCancel"
      width="700px"
    >
      <a-form-model
        ref="ruleForm"
        :model="formData"
        :rules="formRules"
        :label-col="{ span: 5 }"
        :wrapper-col="{ span: 18 }"
      >
        <a-form-model-item label="规则ID" prop="id">
          <a-input v-model="formData.id" placeholder="唯一标识，如 audit_dnslog01" :disabled="isEdit" />
        </a-form-model-item>

        <a-form-model-item label="规则名称" prop="name">
          <a-input v-model="formData.name" placeholder="请输入规则名称" />
        </a-form-model-item>

        <a-form-model-item label="规则描述" prop="description">
          <a-textarea v-model="formData.description" placeholder="请输入规则描述" :rows="2" />
        </a-form-model-item>

        <a-form-model-item label="规则类型" prop="type">
          <a-select v-model="formData.type" placeholder="请选择规则类型">
            <a-select-option value="pattern">模式匹配</a-select-option>
            <a-select-option value="threshold">阈值检测</a-select-option>
            <a-select-option value="correlation">关联分析</a-select-option>
            <a-select-option value="statistical">统计分析</a-select-option>
          </a-select>
        </a-form-model-item>

        <a-form-model-item label="严重程度" prop="severity">
          <a-select v-model="formData.severity" placeholder="请选择严重程度">
            <a-select-option value="critical">严重</a-select-option>
            <a-select-option value="high">高</a-select-option>
            <a-select-option value="medium">中</a-select-option>
            <a-select-option value="low">低</a-select-option>
            <a-select-option value="info">信息</a-select-option>
          </a-select>
        </a-form-model-item>

        <a-form-model-item label="作者" prop="author">
          <a-input v-model="formData.author" placeholder="请输入作者" />
        </a-form-model-item>

        <a-form-model-item label="标签">
          <a-select v-model="formData.tags" mode="tags" placeholder="输入后按 Enter 添加标签" />
        </a-form-model-item>

        <a-form-model-item label="启用状态">
          <a-switch v-model="formData.enabled" checked-children="启用" un-checked-children="禁用" />
        </a-form-model-item>

        <a-form-model-item label="匹配条件" prop="conditionsJson">
          <a-textarea
            v-model="formData.conditionsJson"
            placeholder='JSON 格式，如 {"event_type":"process_create","patterns":{"cmdline":"curl.*\\|.*sh"}}'
            :rows="4"
          />
        </a-form-model-item>

        <a-form-model-item label="动作" prop="actionsJson">
          <a-textarea
            v-model="formData.actionsJson"
            placeholder='JSON 数组格式，如 [{"type":"alert","config":{"level":"high"}}]'
            :rows="3"
          />
        </a-form-model-item>
      </a-form-model>
    </a-modal>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import { getRuleList, createRule, updateRule, deleteRule } from '@/services/ruleManagement'

export default {
  name: 'RuleList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        name: '',
        type: '',
        severity: ''
      },
      loading: false,
      data: [],
      columns: [
        {
          title: '规则ID',
          dataIndex: 'id',
          key: 'id',
          width: 160
        },
        {
          title: '规则名称',
          dataIndex: 'name',
          key: 'name',
          width: 180
        },
        {
          title: '规则类型',
          dataIndex: 'type',
          key: 'type',
          width: 110,
          scopedSlots: { customRender: 'type' }
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
          title: '启用状态',
          dataIndex: 'enabled',
          key: 'enabled',
          width: 100,
          scopedSlots: { customRender: 'enabled' }
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
        total: 0,
        showSizeChanger: true,
        showQuickJumper: true,
        showTotal: total => `共 ${total} 条记录`
      },
      visible: false,
      confirmLoading: false,
      modalTitle: '',
      isEdit: false,
      formData: this.defaultFormData(),
      formRules: {
        id: [{ required: true, message: '请输入规则ID', trigger: 'blur' }],
        name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
        type: [{ required: true, message: '请选择规则类型', trigger: 'change' }],
        severity: [{ required: true, message: '请选择严重程度', trigger: 'change' }],
        conditionsJson: [{ validator: this.validateJsonObject, trigger: 'blur' }],
        actionsJson: [{ validator: this.validateJsonArray, trigger: 'blur' }]
      }
    }
  },
  created() {
    this.fetchRules()
  },
  methods: {
    defaultFormData() {
      return {
        id: '',
        name: '',
        description: '',
        type: undefined,
        severity: undefined,
        author: '',
        tags: [],
        enabled: true,
        conditionsJson: '{}',
        actionsJson: '[]'
      }
    },

    validateJsonObject(rule, value, callback) {
      if (!value) return callback()
      try {
        const parsed = JSON.parse(value)
        if (typeof parsed !== 'object' || Array.isArray(parsed)) {
          return callback(new Error('匹配条件必须是 JSON 对象'))
        }
      } catch (e) {
        return callback(new Error('匹配条件不是合法的 JSON'))
      }
      callback()
    },

    validateJsonArray(rule, value, callback) {
      if (!value) return callback()
      try {
        const parsed = JSON.parse(value)
        if (!Array.isArray(parsed)) {
          return callback(new Error('动作必须是 JSON 数组'))
        }
      } catch (e) {
        return callback(new Error('动作不是合法的 JSON'))
      }
      callback()
    },

    async fetchRules() {
      this.loading = true
      try {
        const params = {}
        if (this.queryParams.name) params.name = this.queryParams.name
        if (this.queryParams.type) params.type = this.queryParams.type
        if (this.queryParams.severity) params.severity = this.queryParams.severity

        const res = await getRuleList(params)
        const rules = (res.data && res.data.rules) || []
        this.data = rules
        this.pagination.total = rules.length
      } catch (error) {
        console.error('获取规则列表失败:', error)
        this.$message.error('获取规则列表失败')
      } finally {
        this.loading = false
      }
    },

    handleSearch() {
      this.pagination.current = 1
      this.fetchRules()
    },

    handleReset() {
      this.queryParams = {
        name: '',
        type: '',
        severity: ''
      }
      this.handleSearch()
    },

    handleAdd() {
      this.modalTitle = '新增规则'
      this.isEdit = false
      this.formData = this.defaultFormData()
      this.visible = true
    },

    handleEdit(record) {
      this.modalTitle = '编辑规则'
      this.isEdit = true
      this.formData = {
        id: record.id,
        name: record.name,
        description: record.description || '',
        type: record.type,
        severity: record.severity,
        author: record.author || '',
        tags: record.tags || [],
        enabled: record.enabled,
        conditionsJson: JSON.stringify(record.conditions || {}, null, 2),
        actionsJson: JSON.stringify(record.actions || [], null, 2)
      }
      this.visible = true
    },

    async handleOk() {
      this.$refs.ruleForm.validate(async valid => {
        if (!valid) return

        this.confirmLoading = true
        try {
          const payload = {
            id: this.formData.id,
            name: this.formData.name,
            description: this.formData.description,
            type: this.formData.type,
            severity: this.formData.severity,
            author: this.formData.author,
            tags: this.formData.tags,
            enabled: this.formData.enabled,
            conditions: JSON.parse(this.formData.conditionsJson || '{}'),
            actions: JSON.parse(this.formData.actionsJson || '[]')
          }

          if (this.isEdit) {
            await updateRule(this.formData.id, payload)
            this.$message.success('编辑成功')
          } else {
            await createRule(payload)
            this.$message.success('新增成功')
          }
          this.visible = false
          this.fetchRules()
        } catch (error) {
          console.error('操作失败:', error)
          this.$message.error((error.response && error.response.data && error.response.data.error) || '操作失败')
        } finally {
          this.confirmLoading = false
        }
      })
    },

    handleCancel() {
      this.visible = false
    },

    async handleDelete(record) {
      try {
        await deleteRule(record.id)
        this.$message.success('删除成功')
        this.fetchRules()
      } catch (error) {
        console.error('删除失败:', error)
        this.$message.error((error.response && error.response.data && error.response.data.error) || '删除失败')
      }
    },

    async handleToggle(record, checked) {
      try {
        await updateRule(record.id, { enabled: checked })
        this.$message.success(`已${checked ? '启用' : '禁用'}规则`)
        this.fetchRules()
      } catch (error) {
        console.error('操作失败:', error)
        this.$message.error((error.response && error.response.data && error.response.data.error) || '操作失败')
      }
    },

    handleTableChange(pagination) {
      this.pagination = pagination
    },

    getTypeColor(type) {
      const colors = {
        pattern: 'blue',
        threshold: 'orange',
        correlation: 'purple',
        statistical: 'green'
      }
      return colors[type] || 'default'
    },

    getTypeText(type) {
      const texts = {
        pattern: '模式匹配',
        threshold: '阈值检测',
        correlation: '关联分析',
        statistical: '统计分析'
      }
      return texts[type] || type
    },

    getSeverityColor(severity) {
      const colors = {
        critical: 'red',
        high: 'orange',
        medium: 'gold',
        low: 'blue',
        info: 'default'
      }
      return colors[severity] || 'default'
    },

    getSeverityText(severity) {
      const texts = {
        critical: '严重',
        high: '高',
        medium: '中',
        low: '低',
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
