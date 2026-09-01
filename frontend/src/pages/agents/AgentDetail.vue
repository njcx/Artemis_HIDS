<template>
  <page-layout>
    <a-card :bordered="false">
      <!-- 返回按钮 -->
      <div class="detail-header">
        <a-button icon="arrow-left" @click="goBack">返回</a-button>
        <h2>{{ agentInfo.id || agentInfo.hostname }} - 主机详情</h2>
      </div>

      <!-- 概览统计卡片 -->
      <div class="agent-overview">
        <a-row :gutter="16">
          <a-col :span="6">
            <div class="stat-card" :class="agentInfo.status === 'online' ? 'online' : 'offline'">
              <div class="stat-icon">
                <a-icon :type="agentInfo.status === 'online' ? 'check-circle' : 'close-circle'" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ agentInfo.status === 'online' ? '在线' : '离线' }}</div>
                <div class="stat-label">主机状态</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card cpu">
              <div class="stat-icon">
                <a-icon type="dashboard" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ formatPercent(agentInfo.cpuLoad) }}</div>
                <div class="stat-label">CPU 负载</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card memory">
              <div class="stat-icon">
                <a-icon type="database" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ formatPercent(agentInfo.memoryPercent) }}</div>
                <div class="stat-label">内存占用</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card disk">
              <div class="stat-icon">
                <a-icon type="hdd" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ formatPercent(agentInfo.diskPercent) }}</div>
                <div class="stat-label">磁盘占用</div>
              </div>
            </div>
          </a-col>
        </a-row>
      </div>

      <!-- 基本信息 -->
      <a-card title="基本信息" class="info-card" :bordered="false">
        <template slot="extra">
          <a-icon type="info-circle" style="color: #1890ff" />
        </template>
        <a-row :gutter="16">
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">Agent ID：</span>
              <span class="info-value" :class="{ 'empty-value': !agentInfo.id }">{{ agentInfo.id || 'N/A' }}</span>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">状态：</span>
              <a-badge :status="agentInfo.status === 'online' ? 'success' : 'error'"
                       :text="agentInfo.status === 'online' ? '在线' : '离线'" />
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">操作系统：</span>
              <a-tag color="blue">{{ agentInfo.os || 'N/A' }}</a-tag>
            </div>
          </a-col>
        </a-row>
        <a-row :gutter="16" style="margin-top: 16px;">
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">最后心跳：</span>
              <span class="info-value">{{ formatTime(agentInfo.lastSeen) }}</span>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">IP 个数：</span>
              <a-tag color="cyan">{{ agentInfo.ipCount || 0 }}</a-tag>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">具体 IP：</span>
              <span class="info-value" :class="{ 'empty-value': !agentInfo.localIps }">{{ agentInfo.localIps || 'N/A' }}</span>
            </div>
          </a-col>
        </a-row>
      </a-card>

      <!-- 硬件信息 -->
      <a-card title="硬件信息" class="info-card" :bordered="false" style="margin-top: 16px;">
        <template slot="extra">
          <a-icon type="laptop" style="color: #722ed1" />
        </template>
        <a-row :gutter="16">
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">CPU 型号：</span>
              <span class="info-value" :class="{ 'empty-value': !agentInfo.cpuModel }">{{ agentInfo.cpuModel || 'N/A' }}</span>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">CPU 个数：</span>
              <a-tag color="purple">{{ agentInfo.cpuCount || 0 }} 核</a-tag>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">CPU 负载：</span>
              <a-progress :percent="parseNumber(agentInfo.cpuLoad)" size="small" :status="getCpuStatus(agentInfo.cpuLoad)" style="width: 120px" />
            </div>
          </a-col>
        </a-row>
        <a-row :gutter="16" style="margin-top: 16px;">
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">内存大小：</span>
              <a-tag color="green">{{ agentInfo.memoryTotal || 'N/A' }}</a-tag>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">内存占用：</span>
              <a-progress :percent="parseNumber(agentInfo.memoryPercent)" size="small" :status="getMemoryStatus(agentInfo.memoryPercent)" style="width: 120px" />
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">磁盘个数：</span>
              <a-tag color="orange">{{ agentInfo.diskCount || 0 }} 个</a-tag>
            </div>
          </a-col>
        </a-row>
        <a-row :gutter="16" style="margin-top: 16px;">
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">磁盘大小：</span>
              <a-tag color="blue">{{ agentInfo.diskTotal || 'N/A' }}</a-tag>
            </div>
          </a-col>
          <a-col :span="8">
            <div class="info-item">
              <span class="info-label">磁盘占比：</span>
              <a-progress :percent="parseNumber(agentInfo.diskPercent)" size="small" :status="getDiskStatus(agentInfo.diskPercent)" style="width: 120px" />
            </div>
          </a-col>
        </a-row>
      </a-card>

      <!-- 安全事件 -->
      <a-card :bordered="false" title="安全事件" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredSecurityEvents.length" :number-style="{ backgroundColor: '#f5222d' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.securityEvents"
            placeholder="搜索事件类型、描述或来源"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="securityEventColumns"
          :data-source="filteredSecurityEvents"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="id"
        >
          <template slot="level" slot-scope="level">
            <a-tag :color="getEventLevelColor(level)">{{ getEventLevelText(level) }}</a-tag>
          </template>
          <template slot="status" slot-scope="status">
            <a-badge :status="status === 'resolved' ? 'success' : status === 'processing' ? 'processing' : 'error'" 
                     :text="status === 'resolved' ? '已处理' : status === 'processing' ? '处理中' : '未处理'" />
          </template>
          <template slot="action" slot-scope="text, record">
            <a @click="handleEventDetail(record)">详情</a>
            <!-- <a-divider type="vertical" />
            <a @click="handleEventProcess(record)" v-if="record.status === 'pending'">处理</a>
            <a disabled v-else>已处理</a> -->
          </template>
        </a-table>
      </a-card>

      <!-- 进程列表 -->
      <a-card :bordered="false" title="进程列表" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredProcesses.length" :number-style="{ backgroundColor: '#52c41a' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.processes"
            placeholder="搜索进程名、用户或命令"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="processColumns"
          :data-source="filteredProcesses"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="pid"
        >
          <template slot="cpu" slot-scope="cpu">
            <span :style="{ color: cpu > 50 ? '#f5222d' : cpu > 20 ? '#fa8c16' : '#52c41a' }">
              {{ cpu }}%
            </span>
          </template>
          <template slot="memory" slot-scope="memory">
            <span :style="{ color: memory > 50 ? '#f5222d' : memory > 20 ? '#fa8c16' : '#52c41a' }">
              {{ memory }}%
            </span>
          </template>
        </a-table>
      </a-card>

      <!-- 端口列表 -->
      <a-card :bordered="false" title="端口列表" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredPorts.length" :number-style="{ backgroundColor: '#1890ff' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.ports"
            placeholder="搜索端口、程序或PID"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="portColumns"
          :data-source="filteredPorts"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="port"
        >
          <template slot="status" slot-scope="status">
            <a-badge :status="status === 'LISTEN' ? 'success' : 'default'" :text="status" />
          </template>
          <template slot="protocol" slot-scope="protocol">
            <a-tag :color="protocol === 'TCP' ? 'blue' : 'green'">{{ protocol }}</a-tag>
          </template>
        </a-table>
      </a-card>

      <!-- 软件资产 -->
      <a-card :bordered="false" title="已安装软件" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredSoftware.length" :number-style="{ backgroundColor: '#722ed1' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.software"
            placeholder="搜索软件名、版本或厂商"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="softwareColumns"
          :data-source="filteredSoftware"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="name"
        />
      </a-card>

      <!-- 系统服务 -->
      <a-card :bordered="false" title="系统服务" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredServices.length" :number-style="{ backgroundColor: '#fa8c16' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.services"
            placeholder="搜索服务名或描述"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="serviceColumns"
          :data-source="filteredServices"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="name"
        >
          <template slot="status" slot-scope="status">
            <a-badge :status="status === 'running' ? 'success' : 'default'" 
                     :text="status === 'running' ? '运行中' : '已停止'" />
          </template>
        </a-table>
      </a-card>

      <!-- 容器列表 -->
      <a-card :bordered="false" title="容器列表" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredContainers.length" :number-style="{ backgroundColor: '#13c2c2' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.containers"
            placeholder="搜索容器名或镜像"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="containerColumns"
          :data-source="filteredContainers"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="id"
        >
          <template slot="status" slot-scope="status">
            <a-badge :status="status === 'running' ? 'success' : 'error'" 
                     :text="status === 'running' ? '运行中' : '已停止'" />
          </template>
          <template slot="image" slot-scope="image">
            <a-tooltip :title="image">
              <a-tag color="cyan">{{ image }}</a-tag>
            </a-tooltip>
          </template>
        </a-table>
      </a-card>

      <!-- 网络连接 -->
      <a-card :bordered="false" title="网络连接" style="margin-top: 16px;">
        <a-badge slot="extra" :count="filteredConnections.length" :number-style="{ backgroundColor: '#eb2f96' }" />
        <div style="margin-bottom: 16px;">
          <a-input-search
            v-model="searchText.connections"
            placeholder="搜索地址、程序或PID"
            style="width: 300px"
            @change="handleSearch"
            allowClear
          />
        </div>
        <a-table
          :columns="connectionColumns"
          :data-source="filteredConnections"
          :pagination="{ pageSize: 10 }"
          size="middle"
          rowKey="id"
        >
          <template slot="state" slot-scope="state">
            <a-tag :color="getConnectionStateColor(state)">{{ state }}</a-tag>
          </template>
        </a-table>
      </a-card>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import {request, METHOD} from '@/utils/request'

export default {
  name: 'AgentDetail',
  components: { PageLayout },
  data() {
    return {
      searchText: {
        securityEvents: '',
        processes: '',
        ports: '',
        software: '',
        services: '',
        containers: '',
        connections: ''
      },
      agentInfo: {
        id: '',
        hostname: '',
        ip: '',
        os: '',
        status: 'offline',
        lastSeen: new Date(),
        // 新增硬件信息字段
        cpuModel: '',
        cpuCount: 0,
        cpuLoad: 0,
        memoryTotal: '',
        memoryPercent: 0,
        diskTotal: '',
        diskCount: 0,
        diskPercent: 0,
        ipCount: 0,
        localIps: ''
      },
      // 进程列表
      processes: [],
      processColumns: [
        { title: 'PID', dataIndex: 'pid', key: 'pid' },
        { title: '进程名', dataIndex: 'name', key: 'name' },
        { title: '用户', dataIndex: 'user', key: 'user' },
        { title: 'CPU', dataIndex: 'cpu', key: 'cpu', scopedSlots: { customRender: 'cpu' } },
        { title: '内存', dataIndex: 'memory', key: 'memory', scopedSlots: { customRender: 'memory' } },
        { title: '命令', dataIndex: 'command', key: 'command' }
      ],
      // 端口列表
      ports: [],
      portColumns: [
        { title: '端口', dataIndex: 'port', key: 'port', width: 100 },
        { title: '协议', dataIndex: 'protocol', key: 'protocol', width: 100, scopedSlots: { customRender: 'protocol' } },
        { title: '状态', dataIndex: 'status', key: 'status', width: 120, scopedSlots: { customRender: 'status' } },
        { title: '程序', dataIndex: 'program', key: 'program', width: 180 },
        { title: 'PID', dataIndex: 'pid', key: 'pid', width: 100 }
      ],
      // 软件资产
      software: [],
      softwareColumns: [
        { title: '软件名', dataIndex: 'name', key: 'name', width: 150 },
        { title: '版本', dataIndex: 'version', key: 'version', width: 130 },
        { title: '厂商', dataIndex: 'vendor', key: 'vendor', width: 180 },
        { title: '安装日期', dataIndex: 'installDate', key: 'installDate', width: 140 }
      ],
      // 系统服务
      services: [],
      serviceColumns: [
        { title: '服务名', dataIndex: 'name', key: 'name', width: 150 },
        { title: '状态', dataIndex: 'status', key: 'status', width: 120, scopedSlots: { customRender: 'status' } },
        { title: '开机启动', dataIndex: 'enabled', key: 'enabled', width: 110, customRender: (enabled) => enabled ? '是' : '否' },
        { title: '描述', dataIndex: 'description', key: 'description', width: 220 }
      ],
      // 容器列表
      containers: [],
      containerColumns: [
        { title: 'ID', dataIndex: 'id', key: 'id', width: 120 },
        { title: '容器名', dataIndex: 'name', key: 'name', width: 160 },
        { title: '镜像', dataIndex: 'image', key: 'image', width: 180, scopedSlots: { customRender: 'image' } },
        { title: '状态', dataIndex: 'status', key: 'status', width: 110, scopedSlots: { customRender: 'status' } },
        { title: '端口映射', dataIndex: 'ports', key: 'ports', width: 160 },
        { title: '创建时间', dataIndex: 'created', key: 'created', width: 120 }
      ],
      // 网络连接
      connections: [],
      connectionColumns: [
        { title: '本地地址', dataIndex: 'localAddr', key: 'localAddr', width: 180 },
        { title: '远程地址', dataIndex: 'remoteAddr', key: 'remoteAddr', width: 180 },
        { title: '状态', dataIndex: 'state', key: 'state', width: 140, scopedSlots: { customRender: 'state' } },
        { title: 'PID', dataIndex: 'pid', key: 'pid', width: 90 },
        { title: '程序', dataIndex: 'program', key: 'program', width: 160 }
      ],
      // 安全事件
      securityEvents: [],
      securityEventColumns: [
        { title: '时间', dataIndex: 'time', key: 'time', width: 160 },
        { title: '级别', dataIndex: 'level', key: 'level', width: 100, scopedSlots: { customRender: 'level' } },
        { title: '事件类型', dataIndex: 'type', key: 'type', width: 120 },
        { title: '描述', dataIndex: 'description', key: 'description', width: 300 },
        { title: '状态', dataIndex: 'status', key: 'status', width: 120, scopedSlots: { customRender: 'status' } },
        { title: '操作', key: 'action', width: 150, scopedSlots: { customRender: 'action' } }
      ]
    }
  },
  computed: {
    filteredSecurityEvents() {
      const text = this.searchText.securityEvents.toLowerCase()
      if (!text) return this.securityEvents
      return this.securityEvents.filter(item => 
        item.type.toLowerCase().includes(text) ||
        item.description.toLowerCase().includes(text) ||
        item.source.toLowerCase().includes(text)
      )
    },
    filteredProcesses() {
      const text = this.searchText.processes.toLowerCase()
      if (!text) return this.processes
      return this.processes.filter(item => 
        item.name.toLowerCase().includes(text) ||
        item.user.toLowerCase().includes(text) ||
        item.command.toLowerCase().includes(text)
      )
    },
    filteredPorts() {
      const text = this.searchText.ports.toLowerCase()
      if (!text) return this.ports
      return this.ports.filter(item => 
        String(item.port).includes(text) ||
        item.program.toLowerCase().includes(text) ||
        String(item.pid).includes(text)
      )
    },
    filteredSoftware() {
      const text = this.searchText.software.toLowerCase()
      if (!text) return this.software
      return this.software.filter(item => 
        item.name.toLowerCase().includes(text) ||
        item.version.toLowerCase().includes(text) ||
        item.vendor.toLowerCase().includes(text)
      )
    },
    filteredServices() {
      const text = this.searchText.services.toLowerCase()
      if (!text) return this.services
      return this.services.filter(item => 
        item.name.toLowerCase().includes(text) ||
        item.description.toLowerCase().includes(text)
      )
    },
    filteredContainers() {
      const text = this.searchText.containers.toLowerCase()
      if (!text) return this.containers
      return this.containers.filter(item => 
        item.name.toLowerCase().includes(text) ||
        item.image.toLowerCase().includes(text)
      )
    },
    filteredConnections() {
      const text = this.searchText.connections.toLowerCase()
      if (!text) return this.connections
      return this.connections.filter(item => 
        item.localAddr.toLowerCase().includes(text) ||
        item.remoteAddr.toLowerCase().includes(text) ||
        item.program.toLowerCase().includes(text) ||
        String(item.pid).includes(text)
      )
    }
  },
  mounted() {
    // 从路由参数获取主机ID
    const agentId = this.$route.params.id
    if (agentId) {
      this.loadAgentDetail(agentId)
    }
  },
  methods: {
    handleSearch() {
      // 搜索会通过计算属性自动触发过滤
    },
    async loadAgentDetail(id) {
      try {
        const response = await request(`/api/v1/agents/${id}`, METHOD.GET)
        const data = response.data
        
        // 解析最后心跳时间
        let lastSeen = new Date()
        if (data.last_heartbeat) {
          // 支持多种格式：ISO 8601 字符串、Unix 时间戳（秒或毫秒）
          if (typeof data.last_heartbeat === 'string') {
            lastSeen = new Date(data.last_heartbeat)
          } else if (typeof data.last_heartbeat === 'number') {
            // 判断是秒还是毫秒
            lastSeen = data.last_heartbeat > 1e12 ? new Date(data.last_heartbeat) : new Date(data.last_heartbeat * 1000)
          }
        }
        
        // 获取系统信息数据（可能在 data 子对象中，也可能在顶层）
        const sysData = data.data || data
        
        // 更新基本信息
        this.agentInfo = {
          id: data.agent_id || id,
          hostname: sysData.hostname || data.hostname || '',
          ip: data.ip || '',
          os: this.formatOS(sysData) || this.formatOS(data),
          status: data.online ? 'online' : 'offline',
          lastSeen: lastSeen,
          // 硬件信息
          cpuModel: sysData.cpu_model || '',
          cpuCount: sysData.cpu_physical_count || 0,
          cpuLoad: this.parseNumber(sysData.cpu_load_percent),
          memoryTotal: sysData.memory_total_gb ? `${sysData.memory_total_gb} GB` : '',
          memoryPercent: this.parseNumber(sysData.memory_percent),
          diskTotal: sysData.disk_total_gb ? `${sysData.disk_total_gb} GB` : '',
          diskCount: sysData.disk_count || 0,
          diskPercent: this.parseNumber(sysData.disk_percent),
          ipCount: sysData.ip_count || 0,
          localIps: Array.isArray(sysData.local_ips) ? sysData.local_ips.join(', ') : (sysData.local_ips || '')
        }

        // 处理进程信息（新API直接返回数组）
        if (data.processes) {
          const processes = Array.isArray(data.processes) ? data.processes : []
          this.processes = processes.map((p, index) => ({
            pid: p.pid || index,
            name: p.name || 'N/A',
            user: p.user || p.username || 'N/A',
            cpu: p.cpu_percent || p.cpu || 0,
            memory: p.memory_percent || p.memory || 0,
            command: p.cmdline || p.command || 'N/A'
          }))
        }

        // 处理端口信息（新API直接返回数组）
        if (data.ports) {
          const ports = Array.isArray(data.ports) ? data.ports : []
          this.ports = ports.map(p => ({
            port: p.port || p.local_port,
            protocol: p.protocol || 'TCP',
            status: p.state || p.status || 'LISTEN',
            program: p.process_name || p.program || p.name || 'N/A',
            pid: p.pid || 0
          }))
        }

        // 处理软件资产（新API直接返回数组）
        if (data.software) {
          const software = Array.isArray(data.software) ? data.software : []
          this.software = software.map(s => ({
            name: s.name || 'N/A',
            version: s.version || 'N/A',
            vendor: s.vendor || s.publisher || 'N/A',
            installDate: s.install_date || s.installed || 'N/A'
          }))
        }

        // 处理系统服务（新API直接返回数组）
        if (data.services) {
          const services = Array.isArray(data.services) ? data.services : []
          this.services = services.map(s => ({
            name: s.name || 'N/A',
            status: s.state || s.status || 'unknown',
            enabled: s.start_mode === 'auto' || s.enabled === true || s.enabled === 'enabled',
            description: s.description || s.display_name || 'N/A'
          }))
        }

        // 处理容器信息（新API直接返回数组）
        if (data.containers) {
          const containers = Array.isArray(data.containers) ? data.containers : []
          this.containers = containers.map(c => ({
            id: c.id || c.container_id || 'N/A',
            name: c.name || c.Names || 'N/A',
            image: c.image || c.Image || 'N/A',
            status: c.status || c.Status || c.state || 'unknown',
            ports: c.ports || c.Ports || 'N/A',
            created: c.created || c.Created || 'N/A'
          }))
        }

        // 处理网络连接（新API直接返回数组）
        if (data.connections) {
          const connections = Array.isArray(data.connections) ? data.connections : []
          this.connections = connections.map((c, index) => ({
            id: c.id || index,
            localAddr: c.local_address || (c.local_port ? `${c.local_ip || '0.0.0.0'}:${c.local_port}` : 'N/A'),
            remoteAddr: c.remote_address || (c.remote_port ? `${c.remote_ip || '0.0.0.0'}:${c.remote_port}` : 'N/A'),
            state: c.state || c.status || 'UNKNOWN',
            pid: c.pid || 0,
            program: c.process_name || c.program || c.process || 'N/A'
          }))
        }

        // 处理安全事件（新API直接返回数组）
        if (data.security_events) {
          const events = Array.isArray(data.security_events) ? data.security_events : []
          this.securityEvents = events.map((e, index) => ({
            id: this.getIdString(e._id) || this.getIdString(e.id) || `event_${index}`,
            _id: e._id,
            time: e.time || (e.timestamp ? new Date(e.timestamp / 1000000).toLocaleString() : new Date().toLocaleString()),
            level: e.level || e.severity || 'low',
            type: e.type || e.event_type || 'N/A',
            description: e.description || e.title || e.message || 'N/A',
            source: e.source || 'N/A',
            status: e.status || 'pending'
          }))
        }

        this.$message.success('主机详情加载成功')
      } catch (error) {
        console.error('加载主机详情失败:', error)
        this.$message.error('加载主机详情失败: ' + (error.response?.data?.error || error.message))
      }
    },
    formatOS(data) {
      // 组合操作系统信息
      const parts = []
      if (data.os) parts.push(data.os)
      if (data.platform && data.platform !== data.os) parts.push(data.platform)
      if (data.version && !parts.some(p => p.includes(data.version))) parts.push(data.version)
      
      if (parts.length > 0) {
        return parts.join(' ')
      }
      return ''
    },
    goBack() {
      this.$router.go(-1)
    },
    formatTime(time) {
      const now = new Date()
      const diff = Math.floor((now - time) / 1000)
      
      if (diff < 60) return '刚刚'
      if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
      if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
      return new Date(time).toLocaleString()
    },
    getConnectionStateColor(state) {
      const colorMap = {
        'ESTABLISHED': 'green',
        'LISTEN': 'blue',
        'TIME_WAIT': 'orange',
        'CLOSE_WAIT': 'red',
        'SYN_SENT': 'cyan',
        'SYN_RECV': 'purple'
      }
      return colorMap[state] || 'default'
    },
    getEventLevelColor(level) {
      const colorMap = {
        'critical': 'red',
        'high': 'orange',
        'medium': 'gold',
        'low': 'blue'
      }
      return colorMap[level] || 'default'
    },
    getEventLevelText(level) {
      const textMap = {
        'critical': '严重',
        'high': '高危',
        'medium': '中危',
        'low': '低危'
      }
      return textMap[level] || level
    },
    handleEventDetail(record) {
      // 跳转到告警详情页
      const alertId = record.id || record._id
      if (alertId) {
        this.$router.push({
          path: `/security/${alertId}`
        })
      } else {
        this.$message.warning('无法获取告警ID，无法跳转到详情页')
      }
    },
    handleEventProcess(record) {
      this.$confirm({
        title: '处理安全事件',
        content: `确认处理事件: ${record.description}?`,
        onOk: () => {
          record.status = 'processing'
          this.$message.success('事件已标记为处理中')
        }
      })
    },
    // 解析数字（支持字符串和数字类型）
    parseNumber(value) {
      if (value === null || value === undefined) return 0
      if (typeof value === 'number') return value
      if (typeof value === 'string') {
        const num = parseFloat(value)
        return isNaN(num) ? 0 : num
      }
      return 0
    },
    // 格式化百分比显示
    formatPercent(value) {
      const num = this.parseNumber(value)
      return `${num.toFixed(1)}%`
    },
    // CPU 负载颜色
    getCpuLoadColor(value) {
      const num = this.parseNumber(value)
      if (num >= 80) return '#f5222d'  // 红色 - 高负载
      if (num >= 50) return '#fa8c16'  // 橙色 - 中等负载
      return '#52c41a'  // 绿色 - 正常
    },
    // 内存占用颜色
    getMemoryColor(value) {
      const num = this.parseNumber(value)
      if (num >= 90) return '#f5222d'  // 红色 - 高占用
      if (num >= 70) return '#fa8c16'  // 橙色 - 中等占用
      return '#52c41a'  // 绿色 - 正常
    },
    // 磁盘占用颜色
    getDiskColor(value) {
      const num = this.parseNumber(value)
      if (num >= 90) return '#f5222d'  // 红色 - 高占用
      if (num >= 70) return '#fa8c16'  // 橙色 - 中等占用
      return '#52c41a'  // 绿色 - 正常
    },
    // CPU 状态（用于进度条）
    getCpuStatus(value) {
      const num = this.parseNumber(value)
      if (num >= 80) return 'exception'
      if (num >= 50) return 'active'
      return 'normal'
    },
    // 内存状态（用于进度条）
    getMemoryStatus(value) {
      const num = this.parseNumber(value)
      if (num >= 90) return 'exception'
      if (num >= 70) return 'active'
      return 'normal'
    },
    // 磁盘状态（用于进度条）
    getDiskStatus(value) {
      const num = this.parseNumber(value)
      if (num >= 90) return 'exception'
      if (num >= 70) return 'active'
      return 'normal'
    },
    // 获取ID字符串（处理MongoDB ObjectID）
    getIdString(id) {
      if (!id) return ''
      if (typeof id === 'string') return id
      if (typeof id === 'object' && id.$oid) return id.$oid
      return String(id)
    }
  }
}
</script>

<style lang="less" scoped>
.detail-header {
  display: flex;
  align-items: center;
  margin-bottom: 24px;
  
  h2 {
    margin: 0 0 0 16px;
    font-size: 20px;
    font-weight: 600;
  }
}

.agent-overview {
  margin-bottom: 24px;
  
  .stat-card {
    display: flex;
    align-items: center;
    padding: 20px;
    border-radius: 8px;
    background: #fff;
    border: 1px solid #f0f0f0;
    transition: all 0.3s ease;
    
    &:hover {
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
      transform: translateY(-2px);
    }
    
    .stat-icon {
      width: 56px;
      height: 56px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 8px;
      margin-right: 16px;
      
      .anticon {
        font-size: 28px;
        color: #fff;
      }
    }
    
    .stat-content {
      flex: 1;
      
      .stat-value {
        font-size: 24px;
        font-weight: 600;
        line-height: 1;
        margin-bottom: 4px;
      }
      
      .stat-label {
        font-size: 14px;
        color: #8c8c8c;
      }
    }
    
    &.online {
      .stat-icon {
        background: linear-gradient(135deg, #52c41a 0%, #73d13d 100%);
      }
      .stat-value {
        color: #52c41a;
      }
    }
    
    &.offline {
      .stat-icon {
        background: linear-gradient(135deg, #ff4d4f 0%, #ff7875 100%);
      }
      .stat-value {
        color: #ff4d4f;
      }
    }
    
    &.cpu {
      .stat-icon {
        background: linear-gradient(135deg, #1890ff 0%, #40a9ff 100%);
      }
      .stat-value {
        color: #1890ff;
      }
    }
    
    &.memory {
      .stat-icon {
        background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);
      }
      .stat-value {
        color: #722ed1;
      }
    }
    
    &.disk {
      .stat-icon {
        background: linear-gradient(135deg, #fa8c16 0%, #ffa940 100%);
      }
      .stat-value {
        color: #fa8c16;
      }
    }
  }
}

.info-card {
  margin-bottom: 24px;
  
  /deep/ .ant-card-head {
    border-bottom: 1px solid #f0f0f0;
    
    .ant-card-head-title {
      font-weight: 600;
    }
  }
  
  /deep/ .ant-card-body {
    padding: 24px;
  }
}

.info-item {
  display: flex;
  align-items: center;
  line-height: 32px;
  
  .info-label {
    font-weight: 500;
    color: #595959;
    margin-right: 8px;
    min-width: 80px;
  }
  
  .info-value {
    color: #262626;
    font-weight: 400;
    word-break: break-all;
    
    &.empty-value {
      color: #bfbfbf;
      font-style: italic;
    }
  }
}

.detail-tabs {
  /deep/ .ant-card-body {
    padding: 0;
  }
  
  /deep/ .ant-tabs-bar {
    margin: 0;
    padding: 0 24px;
    border-bottom: 1px solid #f0f0f0;
  }
  
  /deep/ .ant-tabs-content {
    padding: 24px;
  }
}

.asset-collapse {
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  
  /deep/ .ant-collapse-item {
    border-bottom: 1px solid #f0f0f0;
    
    &:last-child {
      border-bottom: none;
    }
  }
  
  /deep/ .ant-collapse-header {
    font-weight: 500;
  }
}

/deep/ .ant-table-small {
  font-size: 13px;
}

.item-label {
  font-weight: 500;
  color: #595959;
  margin-right: 8px;
  display: inline-block;
  min-width: 70px;
}

/deep/ .ant-list-item {
  padding: 0 !important;
  border: none !important;
}

/deep/ .ant-card-small > .ant-card-head {
  padding: 8px 12px;
  min-height: 36px;
  font-size: 14px;
  font-weight: 500;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
}

/deep/ .ant-card-small > .ant-card-body {
  padding: 12px;
  
  p {
    margin-bottom: 8px;
    line-height: 1.8;
    
    &:last-child {
      margin-bottom: 0;
    }
  }
}

.table-container {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 0;
  margin: 0;
  
  /deep/ .ant-table {
    margin: 0 !important;
  }
  
  /deep/ .ant-table-wrapper {
    width: 100%;
    overflow-x: auto;
  }
  
  /deep/ .ant-table-content {
    overflow-x: auto !important;
  }
  
  /deep/ .ant-spin-nested-loading,
  /deep/ .ant-spin-container {
    width: 100%;
  }
  
  /deep/ .ant-table-thead > tr > th:first-child,
  /deep/ .ant-table-tbody > tr > td:first-child {
    padding-left: 16px !important;
  }
}

</style>

