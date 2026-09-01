import { request, METHOD } from '@/utils/request'

/**
 * 获取仪表盘统计数据
 */
export async function getDashboardStats() {
  return request('/api/v1/stats/dashboard', METHOD.GET)
}
