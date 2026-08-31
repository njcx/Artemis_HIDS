import { request, METHOD } from '@/utils/request'

/**
 * 获取告警列表
 * @param {Object} params
 * @param {string} params.severity - 告警级别 (critical/high/medium/low/all)
 * @param {string} params.status - 告警状态 (open/resolved/ignored/all)
 * @param {number} params.page - 页码
 * @param {number} params.page_size - 每页数量
 */
export async function getAlerts(params) {
  return request('/api/v1/alerts', METHOD.GET, params)
}

/**
 * 获取告警详情
 * @param {number|string} id - 告警ID
 */
export async function getAlertDetail(id) {
  const res = await request(`/api/v1/alerts/${id}`, METHOD.GET)
  return res?.data || res
}

/**
 * 更新告警状态
 * @param {number|string} id - 告警ID
 * @param {Object} data
 * @param {string} data.status - 新状态 (open/investigating/resolved/ignored)
 * @param {string} data.comment - 备注 (可选)
 */
export async function updateAlertStatus(id, data) {
  return request(`/api/v1/alerts/${id}/status`, METHOD.PUT, data)
}
