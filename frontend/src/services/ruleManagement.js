import { request, METHOD } from '@/utils/request'

const API_BASE = '/api/v1'

/**
 * 获取规则列表
 * @param {Object} params - 查询参数
 * @param {string} params.name - 规则名称筛选
 * @param {string} params.severity - 严重程度筛选
 * @param {string} params.type - 规则类型筛选
 * @param {string} params.enabled - 启用状态筛选 ('true'/'false')
 */
export async function getRuleList(params = {}) {
  return request(`${API_BASE}/rules`, METHOD.GET, params)
}

/**
 * 获取规则详情
 * @param {string} id - 规则ID
 */
export async function getRule(id) {
  return request(`${API_BASE}/rules/${id}`, METHOD.GET)
}

/**
 * 创建规则
 * @param {Object} data - 规则数据
 */
export async function createRule(data) {
  return request(`${API_BASE}/rules`, METHOD.POST, data)
}

/**
 * 更新规则
 * @param {string} id - 规则ID
 * @param {Object} data - 规则数据
 */
export async function updateRule(id, data) {
  return request(`${API_BASE}/rules/${id}`, METHOD.PUT, data)
}

/**
 * 删除规则
 * @param {string} id - 规则ID
 */
export async function deleteRule(id) {
  return request(`${API_BASE}/rules/${id}`, METHOD.DELETE)
}

export default {
  getRuleList,
  getRule,
  createRule,
  updateRule,
  deleteRule
}
