<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="table-page-search-wrapper">
        <a-form layout="inline">
          <a-row :gutter="48">
            <a-col :md="8" :sm="24">
              <a-form-item label="用户名">
                <a-input v-model="queryParams.username" placeholder="请输入用户名" />
              </a-form-item>
            </a-col>
            <a-col :md="8" :sm="24">
              <a-form-item label="角色">
                <a-select v-model="queryParams.role" placeholder="请选择角色" style="width: 100%">
                  <a-select-option value="">全部</a-select-option>
                  <a-select-option v-for="role in roleOptions" :key="role.id" :value="role.name">
                    {{ role.display_name }}
                  </a-select-option>
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
        <a-button type="primary" icon="plus" @click="handleAdd">新增用户</a-button>
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
        <template slot="avatar" slot-scope="avatar">
          <a-avatar :src="avatar" icon="user" />
        </template>
        
        <template slot="roles" slot-scope="roles">
          <a-tag v-for="role in roles" :key="role.id || role.name" :color="getRoleColor(role.name)">
            {{ role.display_name || getRoleText(role.name) }}
          </a-tag>
        </template>
        
        <template slot="status" slot-scope="status">
          <a-badge :status="status === 'active' ? 'success' : 'default'" :text="status === 'active' ? '启用' : '禁用'" />
        </template>
        
        <template slot="lastLogin" slot-scope="lastLogin">
          {{ lastLogin ? formatTime(lastLogin) : '从未登录' }}
        </template>
        
        <template slot="action" slot-scope="text, record">
          <a @click="handleEdit(record)">编辑</a>
          <a-divider type="vertical" />
          <a @click="handleRole(record)">角色</a>
          <a-divider type="vertical" />
          <a-dropdown>
            <a class="ant-dropdown-link">
              更多 <a-icon type="down" />
            </a>
            <a-menu slot="overlay">
              <a-menu-item>
                <a @click="handleResetPassword(record)">重置密码</a>
              </a-menu-item>
              <a-menu-item>
                <a @click="handleToggleStatus(record)">
                  {{ record.status === 'active' ? '禁用' : '启用' }}
                </a>
              </a-menu-item>
              <a-menu-item>
                <a style="color: #f5222d" @click="handleDelete(record)">删除</a>
              </a-menu-item>
            </a-menu>
          </a-dropdown>
        </template>
      </a-table>
    </a-card>

    <!-- 新增/编辑用户对话框 -->
    <a-modal
      :title="modalTitle"
      :visible="visible"
      :confirm-loading="confirmLoading"
      @ok="handleOk"
      @cancel="handleCancel"
      width="600px"
    >
      <a-form-model
        ref="userForm"
        :model="formData"
        :rules="rules"
        :label-col="{ span: 6 }"
        :wrapper-col="{ span: 16 }"
      >
        <a-form-model-item label="用户名" prop="username">
          <a-input v-model="formData.username" placeholder="请输入用户名" :disabled="!!formData.id" />
        </a-form-model-item>
        
        <a-form-model-item label="邮箱" prop="email">
          <a-input v-model="formData.email" placeholder="请输入邮箱" />
        </a-form-model-item>
        
        <a-form-model-item label="姓名" prop="fullName">
          <a-input v-model="formData.fullName" placeholder="请输入姓名" />
        </a-form-model-item>
        
        <a-form-model-item label="密码" prop="password" v-if="!formData.id">
          <a-input-password v-model="formData.password" placeholder="请输入密码" />
        </a-form-model-item>
        
        <a-form-model-item label="角色" prop="roleId">
          <a-select v-model="formData.roleId" placeholder="请选择角色">
            <a-select-option v-for="role in roleOptions" :key="role.id" :value="role.id">
              {{ role.display_name }}
            </a-select-option>
          </a-select>
        </a-form-model-item>
        
        <a-form-model-item label="状态">
          <a-switch v-model="formData.active" checked-children="启用" un-checked-children="禁用" />
        </a-form-model-item>
      </a-form-model>
    </a-modal>

    <!-- 角色分配对话框 -->
    <a-modal
      title="分配角色"
      :visible="roleVisible"
      :confirm-loading="roleLoading"
      @ok="handleRoleOk"
      @cancel="handleRoleCancel"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="用户">
          <span>{{ currentUser ? currentUser.full_name : '' }}</span>
        </a-form-item>
        <a-form-item label="选择角色">
          <a-radio-group v-model="selectedRole">
            <a-row>
              <a-col :span="24" v-for="role in roleOptions" :key="role.id" style="margin-bottom: 8px">
                <a-radio :value="role.id">
                  {{ role.display_name }}
                  <span style="color: #999; font-size: 12px; margin-left: 8px">{{ role.description }}</span>
                </a-radio>
              </a-col>
            </a-row>
          </a-radio-group>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 重置密码对话框 -->
    <a-modal
      title="重置密码"
      :visible="resetPasswordVisible"
      :confirm-loading="resetPasswordLoading"
      @ok="handleResetPasswordOk"
      @cancel="resetPasswordVisible = false"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="用户">
          <span>{{ currentUser ? currentUser.username : '' }}</span>
        </a-form-item>
        <a-form-item label="新密码">
          <a-input-password v-model="newPassword" placeholder="请输入新密码" />
        </a-form-item>
        <a-form-item label="确认密码">
          <a-input-password v-model="confirmPassword" placeholder="请再次输入新密码" />
        </a-form-item>
      </a-form>
    </a-modal>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import { getUserList, createUser, updateUser, deleteUser, assignRoles, getRoleList, resetUserPassword } from '@/services/userManagement'

export default {
  name: 'UserList',
  components: { PageLayout },
  data() {
    return {
      queryParams: {
        username: '',
        role: ''
      },
      loading: false,
      selectedRowKeys: [],
      data: [],
      roleOptions: [],
      columns: [
        {
          title: '头像',
          dataIndex: 'avatar',
          key: 'avatar',
          width: 80,
          scopedSlots: { customRender: 'avatar' }
        },
        {
          title: '用户名',
          dataIndex: 'username',
          key: 'username',
          width: 120
        },
        {
          title: '姓名',
          dataIndex: 'full_name',
          key: 'full_name',
          width: 120
        },
        {
          title: '邮箱',
          dataIndex: 'email',
          key: 'email'
        },
        {
          title: '角色',
          dataIndex: 'roles',
          key: 'roles',
          width: 200,
          scopedSlots: { customRender: 'roles' }
        },
        {
          title: '状态',
          dataIndex: 'status',
          key: 'status',
          width: 100,
          scopedSlots: { customRender: 'status' }
        },
        {
          title: '最后登录',
          dataIndex: 'last_login',
          key: 'last_login',
          width: 150,
          scopedSlots: { customRender: 'lastLogin' }
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
      visible: false,
      confirmLoading: false,
      modalTitle: '',
      formData: {
        id: null,
        username: '',
        email: '',
        fullName: '',
        password: '',
        roleId: null,
        active: true
      },
      rules: {
        username: [
          { required: true, message: '请输入用户名', trigger: 'blur' },
          { min: 3, max: 20, message: '用户名长度在3-20个字符', trigger: 'blur' }
        ],
        email: [
          { required: true, message: '请输入邮箱', trigger: 'blur' },
          { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' }
        ],
        fullName: [
          { required: true, message: '请输入姓名', trigger: 'blur' }
        ],
        password: [
          { required: true, message: '请输入密码', trigger: 'blur' },
          { min: 6, message: '密码至少6个字符', trigger: 'blur' }
        ]
      },
      roleVisible: false,
      roleLoading: false,
      currentUser: null,
      selectedRole: null,
      // 重置密码相关
      resetPasswordVisible: false,
      resetPasswordLoading: false,
      newPassword: '',
      confirmPassword: ''
    }
  },
  created() {
    this.fetchRoles()
    this.fetchUsers()
  },
  methods: {
    // 获取角色列表
    async fetchRoles() {
      try {
        const res = await getRoleList()
        if (res.data && res.data.roles) {
          this.roleOptions = res.data.roles.map(role => ({
            id: this.getIdString(role.id),
            name: role.name,
            display_name: role.display_name,
            description: role.description
          }))
        }
      } catch (error) {
        console.error('获取角色列表失败:', error)
        this.$message.error('获取角色列表失败')
      }
    },
    
    // 获取用户列表
    async fetchUsers() {
      this.loading = true
      try {
        const res = await getUserList(this.queryParams)
        if (res.data && res.data.users) {
          this.data = res.data.users.map(user => ({
            ...user,
            id: this.getIdString(user.id),
            roles: user.roles || []
          }))
          this.pagination.total = this.data.length
        }
      } catch (error) {
        console.error('获取用户列表失败:', error)
        this.$message.error('获取用户列表失败')
      } finally {
        this.loading = false
      }
    },
    
    // 获取ID字符串（处理MongoDB ObjectID）
    getIdString(id) {
      if (!id) return ''
      if (typeof id === 'string') return id
      if (typeof id === 'object' && id.$oid) return id.$oid
      return String(id)
    },
    
    handleSearch() {
      this.pagination.current = 1
      this.fetchUsers()
    },
    
    handleReset() {
      this.queryParams = {
        username: '',
        role: ''
      }
      this.handleSearch()
    },
    
    onSelectChange(selectedRowKeys) {
      this.selectedRowKeys = selectedRowKeys
    },
    
    handleAdd() {
      this.modalTitle = '新增用户'
      this.formData = {
        id: null,
        username: '',
        email: '',
        fullName: '',
        password: '',
        roleId: null,
        active: true
      }
      this.visible = true
    },
    
    handleEdit(record) {
      this.modalTitle = '编辑用户'
      const roles = record.roles || []
      this.formData = {
        id: record.id,
        username: record.username,
        email: record.email,
        fullName: record.full_name,
        password: '',
        roleId: roles.length > 0 ? this.getIdString(roles[0].id) : null,
        active: record.status === 'active'
      }
      this.visible = true
    },
    
    async handleOk() {
      this.$refs.userForm.validate(async valid => {
        if (valid) {
          this.confirmLoading = true
          try {
            if (this.formData.id) {
              // 更新用户
              await updateUser(this.formData.id, {
                email: this.formData.email,
                full_name: this.formData.fullName,
                status: this.formData.active ? 'active' : 'disabled'
              })
              // 如果角色有变化，更新角色
              if (this.formData.roleId) {
                await assignRoles(this.formData.id, [this.formData.roleId])
              }
              this.$message.success('编辑成功')
            } else {
              // 创建用户
              const res = await createUser({
                username: this.formData.username,
                password: this.formData.password,
                email: this.formData.email,
                full_name: this.formData.fullName
              })
              // 如果有选择角色，分配角色
              if (this.formData.roleId && res.data && res.data.id) {
                const newUserId = this.getIdString(res.data.id)
                await assignRoles(newUserId, [this.formData.roleId])
              }
              this.$message.success('新增成功')
            }
            this.visible = false
            this.fetchUsers()
          } catch (error) {
            console.error('操作失败:', error)
            this.$message.error(error.response?.data?.error || '操作失败')
          } finally {
            this.confirmLoading = false
          }
        }
      })
    },
    
    handleCancel() {
      this.visible = false
    },
    
    handleRole(record) {
      this.currentUser = record
      const roles = record.roles || []
      this.selectedRole = roles.length > 0 ? this.getIdString(roles[0].id) : null
      this.roleVisible = true
    },
    
    async handleRoleOk() {
      if (!this.selectedRole) {
        this.$message.warning('请选择一个角色')
        return
      }
      this.roleLoading = true
      try {
        await assignRoles(this.currentUser.id, [this.selectedRole])
        this.$message.success('角色分配成功')
        this.roleVisible = false
        this.fetchUsers()
      } catch (error) {
        console.error('角色分配失败:', error)
        this.$message.error(error.response?.data?.error || '角色分配失败')
      } finally {
        this.roleLoading = false
      }
    },
    
    handleRoleCancel() {
      this.roleVisible = false
    },
    
    handleResetPassword(record) {
      this.currentUser = record
      this.newPassword = ''
      this.confirmPassword = ''
      this.resetPasswordVisible = true
    },
    
    async handleResetPasswordOk() {
      if (!this.newPassword || this.newPassword.length < 6) {
        this.$message.error('密码至少6个字符')
        return
      }
      if (this.newPassword !== this.confirmPassword) {
        this.$message.error('两次输入的密码不一致')
        return
      }
      
      this.resetPasswordLoading = true
      try {
        await resetUserPassword(this.currentUser.id, this.newPassword)
        this.$message.success('密码重置成功')
        this.resetPasswordVisible = false
      } catch (error) {
        console.error('重置密码失败:', error)
        this.$message.error(error.response?.data?.error || '重置密码失败')
      } finally {
        this.resetPasswordLoading = false
      }
    },
    
    async handleToggleStatus(record) {
      const action = record.status === 'active' ? '禁用' : '启用'
      const newStatus = record.status === 'active' ? 'disabled' : 'active'
      
      this.$confirm({
        title: `${action}用户`,
        content: `确定要${action}用户 ${record.username} 吗？`,
        onOk: async () => {
          try {
            await updateUser(record.id, { status: newStatus })
            this.$message.success(`${action}成功`)
            this.fetchUsers()
          } catch (error) {
            console.error('操作失败:', error)
            this.$message.error(error.response?.data?.error || '操作失败')
          }
        }
      })
    },
    
    handleDelete(record) {
      this.$confirm({
        title: '删除用户',
        content: `确定要删除用户 ${record.username} 吗？此操作不可恢复！`,
        okType: 'danger',
        onOk: async () => {
          try {
            await deleteUser(record.id)
            this.$message.success('删除成功')
            this.fetchUsers()
          } catch (error) {
            console.error('删除失败:', error)
            this.$message.error(error.response?.data?.error || '删除失败')
          }
        }
      })
    },
    
    handleTableChange(pagination) {
      this.pagination = pagination
    },
    
    getRoleColor(role) {
      const colors = {
        admin: 'red',
        operator: 'blue',
        analyst: 'orange',
        auditor: 'orange',
        viewer: 'green'
      }
      return colors[role] || 'default'
    },
    
    getRoleText(role) {
      const texts = {
        admin: '管理员',
        operator: '运维人员',
        analyst: '安全分析师',
        auditor: '审计员',
        viewer: '只读用户'
      }
      return texts[role] || role
    },
    
    formatTime(time) {
      if (!time) return '从未登录'
      return new Date(time).toLocaleString()
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
