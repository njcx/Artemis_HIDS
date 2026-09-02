import { request, METHOD } from '@/utils/request'

const API_BASE = '/api/v1'

/**
 * 获取端口资产列表
 * @param {Object} params - 查询参数
 * @param {string} params.agent_id - 按主机过滤
 * @param {string} params.search - 按进程名模糊搜索
 * @param {number} params.page - 页码
 * @param {number} params.page_size - 每页数量
 */
export async function getAssetPorts(params = {}) {
  return request(`${API_BASE}/assets/ports`, METHOD.GET, params)
}

/**
 * 获取软件资产列表
 */
export async function getAssetSoftware(params = {}) {
  return request(`${API_BASE}/assets/software`, METHOD.GET, params)
}

/**
 * 获取容器资产列表
 */
export async function getAssetContainers(params = {}) {
  return request(`${API_BASE}/assets/containers`, METHOD.GET, params)
}

/**
 * 获取进程资产列表
 */
export async function getAssetProcesses(params = {}) {
  return request(`${API_BASE}/assets/processes`, METHOD.GET, params)
}

export default {
  getAssetPorts,
  getAssetSoftware,
  getAssetContainers,
  getAssetProcesses
}
