import { request, METHOD } from '@/utils/request'

const API_BASE = '/api/v1'

/**
 * 获取用户列表
 * @param {Object} params - 查询参数
 * @param {string} params.username - 用户名筛选
 * @param {string} params.role - 角色筛选
 * @param {number} params.page - 页码
 * @param {number} params.page_size - 每页数量
 */
export async function getUserList(params = {}) {
  return request(`${API_BASE}/users`, METHOD.GET, params)
}

/**
 * 获取用户详情
 * @param {string} id - 用户ID
 */
export async function getUser(id) {
  return request(`${API_BASE}/users/${id}`, METHOD.GET)
}

/**
 * 创建用户
 * @param {Object} data - 用户数据
 * @param {string} data.username - 用户名
 * @param {string} data.password - 密码
 * @param {string} data.email - 邮箱
 * @param {string} data.full_name - 姓名
 */
export async function createUser(data) {
  return request(`${API_BASE}/users`, METHOD.POST, data)
}

/**
 * 更新用户
 * @param {string} id - 用户ID
 * @param {Object} data - 用户数据
 * @param {string} data.email - 邮箱
 * @param {string} data.full_name - 姓名
 * @param {string} data.status - 状态 (active/disabled)
 */
export async function updateUser(id, data) {
  return request(`${API_BASE}/users/${id}`, METHOD.PUT, data)
}

/**
 * 删除用户
 * @param {string} id - 用户ID
 */
export async function deleteUser(id) {
  return request(`${API_BASE}/users/${id}`, METHOD.DELETE)
}

/**
 * 分配角色
 * @param {string} userId - 用户ID
 * @param {string[]} roleIds - 角色ID列表
 */
export async function assignRoles(userId, roleIds) {
  return request(`${API_BASE}/users/${userId}/roles`, METHOD.POST, { role_ids: roleIds })
}

/**
 * 获取角色列表
 */
export async function getRoleList() {
  return request(`${API_BASE}/roles`, METHOD.GET)
}

/**
 * 获取角色详情
 * @param {string} id - 角色ID
 */
export async function getRole(id) {
  return request(`${API_BASE}/roles/${id}`, METHOD.GET)
}

/**
 * 创建角色
 * @param {Object} data - 角色数据
 */
export async function createRole(data) {
  return request(`${API_BASE}/roles`, METHOD.POST, data)
}

/**
 * 更新角色
 * @param {string} id - 角色ID
 * @param {Object} data - 角色数据
 */
export async function updateRole(id, data) {
  return request(`${API_BASE}/roles/${id}`, METHOD.PUT, data)
}

/**
 * 删除角色
 * @param {string} id - 角色ID
 */
export async function deleteRole(id) {
  return request(`${API_BASE}/roles/${id}`, METHOD.DELETE)
}

/**
 * 重置用户密码
 * @param {string} id - 用户ID
 * @param {string} newPassword - 新密码
 */
export async function resetUserPassword(id, newPassword) {
  return request(`${API_BASE}/users/${id}/reset-password`, METHOD.POST, { new_password: newPassword })
}

export default {
  getUserList,
  getUser,
  createUser,
  updateUser,
  deleteUser,
  assignRoles,
  getRoleList,
  getRole,
  createRole,
  updateRole,
  deleteRole,
  resetUserPassword
}
