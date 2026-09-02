import { request, METHOD } from '@/utils/request'

const API_BASE = '/api/v1'

/**
 * 升级 agent 主程序
 * @param {string} agentId - Agent ID
 * @param {Object} data - 升级参数
 * @param {string} data.version - 目标版本号
 * @param {string} data.download_url - 新版本下载地址
 * @param {string} data.checksum - 新版本文件 sha256 校验值
 */
export async function updateAgent(agentId, data) {
  return request(`${API_BASE}/config/agents/${agentId}/update`, METHOD.POST, data)
}

/**
 * 远程卸载 agent
 * @param {string} agentId - Agent ID
 */
export async function uninstallAgent(agentId) {
  return request(`${API_BASE}/config/agents/${agentId}/uninstall`, METHOD.POST)
}

/**
 * 批量下发命令
 * @param {string[]} agentIds - Agent ID 列表
 * @param {string} commandType - 命令类型，如 update_agent / uninstall_agent
 * @param {Object} payload - 命令参数
 */
export async function batchCommand(agentIds, commandType, payload = {}) {
  return request(`${API_BASE}/config/batch`, METHOD.POST, {
    agent_ids: agentIds,
    command_type: commandType,
    payload
  })
}

/**
 * 查询命令执行结果
 * @param {string} commandId - 命令ID
 * @param {string} agentId - 命令所属的 Agent ID（后端要求必填）
 */
export async function getCommandResult(commandId, agentId) {
  return request(`${API_BASE}/config/commands/${commandId}/result`, METHOD.GET, { agent_id: agentId })
}

export default {
  updateAgent,
  uninstallAgent,
  batchCommand,
  getCommandResult
}
