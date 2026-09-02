<template>
  <page-layout>
    <a-card :bordered="false">
      <div class="asset-header">
        <a-row :gutter="16" style="margin-bottom: 24px">
          <a-col :span="6">
            <a-select
              v-model="selectedHost"
              placeholder="选择主机"
              style="width: 100%"
              show-search
              @change="handleHostChange"
            >
              <a-select-option value="all">全部主机</a-select-option>
              <a-select-option v-for="host in hosts" :key="host.id" :value="host.id">
                {{ host.hostname }} ({{ host.ip }})
              </a-select-option>
            </a-select>
          </a-col>
          <a-col :span="18">
            <a-button type="primary" icon="scan" @click="handleScan">
              执行资产扫描
            </a-button>
            <a-button icon="reload" style="margin-left: 8px" @click="handleRefresh">
              刷新
            </a-button>
            <a-button icon="export" style="margin-left: 8px" @click="handleExport">
              导出报告
            </a-button>
          </a-col>
        </a-row>

        <a-row :gutter="16" class="asset-stats">
          <a-col :span="6">
            <div class="stat-card">
              <div class="stat-icon" style="background: linear-gradient(135deg, #1890ff 0%, #40a9ff 100%);">
                <a-icon type="api" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ portsTotal }}</div>
                <div class="stat-label">开放端口</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card">
              <div class="stat-icon" style="background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);">
                <a-icon type="file-zip" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ softwareTotal }}</div>
                <div class="stat-label">软件包</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card">
              <div class="stat-icon" style="background: linear-gradient(135deg, #13c2c2 0%, #5cdbd3 100%);">
                <a-icon type="container" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ containersTotal }}</div>
                <div class="stat-label">容器</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card">
              <div class="stat-icon" style="background: linear-gradient(135deg, #fa8c16 0%, #ffa940 100%);">
                <a-icon type="code" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.dependencies }}</div>
                <div class="stat-label">依赖库</div>
              </div>
            </div>
          </a-col>
        </a-row>
      </div>

      <a-tabs v-model="activeTab">
        <!-- 监听端口 -->
        <a-tab-pane key="ports" tab="监听端口">
          <a-input-search
            v-model="portsSearch"
            placeholder="搜索进程名"
            style="width: 300px; margin-bottom: 16px"
            @search="fetchPorts"
          />
          <a-table
            :columns="portColumns"
            :data-source="portsData"
            :loading="portsLoading"
            :pagination="portsPagination"
            rowKey="id"
            @change="handlePortsTableChange"
          >
            <template slot="port" slot-scope="port, record">
              <a-tag color="blue">{{ port }}/{{ record.protocol }}</a-tag>
            </template>
          </a-table>
        </a-tab-pane>

        <!-- 运行服务 -->
        <a-tab-pane key="services" tab="运行服务">
          <a-input-search
            v-model="servicesSearch"
            placeholder="搜索服务名称"
            style="width: 300px; margin-bottom: 16px"
            @search="handleServicesSearch"
          />
          <a-table
            :columns="serviceColumns"
            :data-source="servicesData"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="id"
          >
            <template slot="status" slot-scope="status">
              <a-badge :status="status === 'running' ? 'success' : 'error'" :text="status === 'running' ? '运行中' : '已停止'" />
            </template>

            <template slot="autoStart" slot-scope="autoStart">
              <a-icon :type="autoStart ? 'check-circle' : 'close-circle'" :style="{ color: autoStart ? '#52c41a' : '#d9d9d9' }" />
            </template>
          </a-table>
        </a-tab-pane>

        <!-- 软件包 -->
        <a-tab-pane key="packages" tab="软件包">
          <div style="margin-bottom: 16px">
            <a-input-search
              v-model="packagesSearch"
              placeholder="搜索软件包名称"
              style="width: 300px"
              @search="fetchSoftware"
            />
          </div>
          <a-table
            :columns="packageColumns"
            :data-source="packagesData"
            :loading="softwareLoading"
            :pagination="softwarePagination"
            rowKey="id"
            @change="handleSoftwareTableChange"
          >
          </a-table>
        </a-tab-pane>

        <!-- 容器 -->
        <a-tab-pane key="containers" tab="容器">
          <a-input-search
            v-model="containersSearch"
            placeholder="搜索容器名/镜像"
            style="width: 300px; margin-bottom: 16px"
            @search="fetchContainers"
          />
          <a-table
            :columns="containerColumns"
            :data-source="containersData"
            :loading="containersLoading"
            :pagination="containersPagination"
            rowKey="id"
            @change="handleContainersTableChange"
          >
            <template slot="status" slot-scope="status">
              <a-badge :status="status === 'running' ? 'success' : 'default'" :text="status" />
            </template>
          </a-table>
        </a-tab-pane>

        <!-- Java应用 -->
        <a-tab-pane key="java" tab="Java应用">
          <a-input-search
            v-model="javaSearch"
            placeholder="搜索JAR包名称"
            style="width: 300px; margin-bottom: 16px"
            @search="handleJavaSearch"
          />
          <a-table
            :columns="javaColumns"
            :data-source="javaData"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="id"
          >
            <template slot="version" slot-scope="version, record">
              <div>
                <div>{{ version }}</div>
                <div style="font-size: 12px; color: #8c8c8c">JDK {{ record.jdkVersion }}</div>
              </div>
            </template>

            <template slot="vulnerabilities" slot-scope="vulnerabilities">
              <a-tag v-if="vulnerabilities > 0" color="red">
                {{ vulnerabilities }} 个漏洞
              </a-tag>
              <a-tag v-else color="green">
                无已知漏洞
              </a-tag>
            </template>
          </a-table>
        </a-tab-pane>

        <!-- 依赖库 -->
        <a-tab-pane key="dependencies" tab="依赖库">
          <div style="margin-bottom: 16px">
            <a-radio-group v-model="depType" button-style="solid" @change="handleDepTypeChange">
              <a-radio-button value="all">全部</a-radio-button>
              <a-radio-button value="python">Python</a-radio-button>
              <a-radio-button value="node">Node.js</a-radio-button>
              <a-radio-button value="java">Java</a-radio-button>
              <a-radio-button value="go">Go</a-radio-button>
            </a-radio-group>
            <a-input-search
              v-model="depsSearch"
              placeholder="搜索依赖库名称"
              style="width: 300px; margin-left: 16px"
              @search="handleDepsSearch"
            />
          </div>
          <a-table
            :columns="depColumns"
            :data-source="depsData"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="id"
          >
            <template slot="type" slot-scope="type">
              <a-tag :color="getDepTypeColor(type)">
                {{ type }}
              </a-tag>
            </template>

            <template slot="license" slot-scope="license">
              <span style="color: #595959">{{ license }}</span>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'
import { request, METHOD } from '@/utils/request'
import { getAssetPorts, getAssetSoftware, getAssetContainers } from '@/services/assetManagement'

export default {
  name: 'AssetDiscovery',
  components: { PageLayout },
  data() {
    return {
      selectedHost: 'all',
      hosts: [],
      stats: {
        dependencies: 328
      },
      activeTab: 'ports',
      loading: false,
      servicesSearch: '',
      javaSearch: '',
      depsSearch: '',
      depType: 'all',

      // 端口数据（真实接口）
      portsSearch: '',
      portsLoading: false,
      portsData: [],
      portsTotal: 0,
      portsPagination: {
        current: 1,
        pageSize: 10,
        total: 0,
        showSizeChanger: true,
        showTotal: total => `共 ${total} 条记录`
      },
      portColumns: [
        { title: '主机', dataIndex: 'agent_id', key: 'agent_id', width: 220 },
        { title: '端口', dataIndex: 'port', key: 'port', width: 100, scopedSlots: { customRender: 'port' } },
        { title: '进程', dataIndex: 'process_name', key: 'process_name', width: 150 },
        { title: '本地地址', dataIndex: 'local_address', key: 'local_address', width: 150 },
        { title: 'PID', dataIndex: 'pid', key: 'pid', width: 100 }
      ],

      // 服务数据（保持 mock，Agent 侧暂无独立服务采集器）
      servicesData: [
        { id: 1, hostname: 'web-server-01', name: 'nginx', description: 'Nginx Web Server', status: 'running', autoStart: true, version: '1.18.0' },
        { id: 2, hostname: 'web-server-01', name: 'docker', description: 'Docker Engine', status: 'running', autoStart: true, version: '20.10.8' },
        { id: 3, hostname: 'db-server-02', name: 'mysql', description: 'MySQL Database', status: 'running', autoStart: true, version: '8.0.26' },
        { id: 4, hostname: 'app-server-03', name: 'redis', description: 'Redis Cache', status: 'running', autoStart: true, version: '6.2.5' }
      ],
      serviceColumns: [
        { title: '主机', dataIndex: 'hostname', key: 'hostname', width: 150 },
        { title: '服务名称', dataIndex: 'name', key: 'name', width: 150 },
        { title: '描述', dataIndex: 'description', key: 'description' },
        { title: '版本', dataIndex: 'version', key: 'version', width: 120 },
        { title: '状态', dataIndex: 'status', key: 'status', width: 100, scopedSlots: { customRender: 'status' } },
        { title: '开机自启', dataIndex: 'autoStart', key: 'autoStart', width: 100, scopedSlots: { customRender: 'autoStart' } }
      ],

      // 软件包数据（真实接口）
      packagesSearch: '',
      softwareLoading: false,
      packagesData: [],
      softwareTotal: 0,
      softwarePagination: {
        current: 1,
        pageSize: 10,
        total: 0,
        showSizeChanger: true,
        showTotal: total => `共 ${total} 条记录`
      },
      packageColumns: [
        { title: '主机', dataIndex: 'agent_id', key: 'agent_id', width: 220 },
        { title: '包名', dataIndex: 'name', key: 'name', width: 200 },
        { title: '版本', dataIndex: 'version', key: 'version', width: 150 },
        { title: '包管理器', dataIndex: 'package_manager', key: 'package_manager', width: 120 },
        { title: '架构', dataIndex: 'architecture', key: 'architecture', width: 100 }
      ],

      // 容器数据（真实接口，新增 Tab）
      containersSearch: '',
      containersLoading: false,
      containersData: [],
      containersTotal: 0,
      containersPagination: {
        current: 1,
        pageSize: 10,
        total: 0,
        showSizeChanger: true,
        showTotal: total => `共 ${total} 条记录`
      },
      containerColumns: [
        { title: '主机', dataIndex: 'agent_id', key: 'agent_id', width: 220 },
        { title: '容器名', dataIndex: 'name', key: 'name', width: 180 },
        { title: '镜像', dataIndex: 'image', key: 'image' },
        { title: '状态', dataIndex: 'status', key: 'status', width: 120, scopedSlots: { customRender: 'status' } },
        { title: '端口映射', dataIndex: 'ports', key: 'ports', width: 200 }
      ],

      // Java应用数据（保持 mock，Agent 侧暂无对应采集器）
      javaData: [
        { id: 1, hostname: 'app-server-03', name: 'spring-boot-app.jar', version: '2.5.4', jdkVersion: '11', path: '/opt/app/spring-boot-app.jar', vulnerabilities: 0 },
        { id: 2, hostname: 'app-server-03', name: 'tomcat', version: '9.0.50', jdkVersion: '8', path: '/usr/local/tomcat', vulnerabilities: 2 },
        { id: 3, hostname: 'web-server-01', name: 'kafka', version: '2.8.0', jdkVersion: '11', path: '/opt/kafka', vulnerabilities: 0 }
      ],
      javaColumns: [
        { title: '主机', dataIndex: 'hostname', key: 'hostname', width: 150 },
        { title: 'JAR包/应用', dataIndex: 'name', key: 'name', width: 200 },
        { title: '版本', key: 'version', width: 150, scopedSlots: { customRender: 'version' } },
        { title: '路径', dataIndex: 'path', key: 'path', ellipsis: true },
        { title: '漏洞', dataIndex: 'vulnerabilities', key: 'vulnerabilities', width: 120, scopedSlots: { customRender: 'vulnerabilities' } }
      ],

      // 依赖库数据（保持 mock，Agent 侧暂无对应采集器）
      depsData: [
        { id: 1, hostname: 'app-server-03', name: 'django', version: '3.2.7', type: 'Python', license: 'BSD' },
        { id: 2, hostname: 'app-server-03', name: 'flask', version: '2.0.1', type: 'Python', license: 'BSD' },
        { id: 3, hostname: 'web-server-01', name: 'react', version: '17.0.2', type: 'Node.js', license: 'MIT' },
        { id: 4, hostname: 'web-server-01', name: 'vue', version: '3.2.20', type: 'Node.js', license: 'MIT' },
        { id: 5, hostname: 'app-server-03', name: 'spring-core', version: '5.3.9', type: 'Java', license: 'Apache 2.0' }
      ],
      depColumns: [
        { title: '主机', dataIndex: 'hostname', key: 'hostname', width: 150 },
        { title: '依赖库', dataIndex: 'name', key: 'name', width: 180 },
        { title: '版本', dataIndex: 'version', key: 'version', width: 120 },
        { title: '类型', dataIndex: 'type', key: 'type', width: 100, scopedSlots: { customRender: 'type' } },
        { title: '许可证', dataIndex: 'license', key: 'license', width: 120, scopedSlots: { customRender: 'license' } }
      ]
    }
  },
  created() {
    this.fetchHosts()
    this.fetchPorts()
    this.fetchSoftware()
    this.fetchContainers()
  },
  methods: {
    async fetchHosts() {
      try {
        const res = await request('/api/v1/agents', METHOD.GET)
        const agents = (res.data && res.data.agents) || []
        this.hosts = agents.map(agent => ({
          id: agent.agent_id,
          hostname: agent.hostname || agent.agent_id,
          ip: agent.ip || 'N/A'
        }))
      } catch (error) {
        console.error('获取主机列表失败:', error)
      }
    },

    currentAgentFilter() {
      return this.selectedHost === 'all' ? '' : this.selectedHost
    },

    async fetchPorts() {
      this.portsLoading = true
      try {
        const params = {
          page: this.portsPagination.current,
          page_size: this.portsPagination.pageSize
        }
        if (this.portsSearch) params.search = this.portsSearch
        const agentID = this.currentAgentFilter()
        if (agentID) params.agent_id = agentID

        const res = await getAssetPorts(params)
        const items = (res.data && res.data.items) || []
        this.portsData = items.map(item => ({ ...item, id: item.id || `${item.agent_id}-${item.port}` }))
        this.portsTotal = (res.data && res.data.total) || 0
        this.portsPagination.total = this.portsTotal
      } catch (error) {
        console.error('获取端口资产失败:', error)
        this.$message.error('获取端口资产失败')
      } finally {
        this.portsLoading = false
      }
    },

    async fetchSoftware() {
      this.softwareLoading = true
      try {
        const params = {
          page: this.softwarePagination.current,
          page_size: this.softwarePagination.pageSize
        }
        if (this.packagesSearch) params.search = this.packagesSearch
        const agentID = this.currentAgentFilter()
        if (agentID) params.agent_id = agentID

        const res = await getAssetSoftware(params)
        const items = (res.data && res.data.items) || []
        this.packagesData = items.map(item => ({ ...item, id: item.id || `${item.agent_id}-${item.name}-${item.version}` }))
        this.softwareTotal = (res.data && res.data.total) || 0
        this.softwarePagination.total = this.softwareTotal
      } catch (error) {
        console.error('获取软件资产失败:', error)
        this.$message.error('获取软件资产失败')
      } finally {
        this.softwareLoading = false
      }
    },

    async fetchContainers() {
      this.containersLoading = true
      try {
        const params = {
          page: this.containersPagination.current,
          page_size: this.containersPagination.pageSize
        }
        if (this.containersSearch) params.search = this.containersSearch
        const agentID = this.currentAgentFilter()
        if (agentID) params.agent_id = agentID

        const res = await getAssetContainers(params)
        const items = (res.data && res.data.items) || []
        this.containersData = items.map(item => ({ ...item, id: item.id || `${item.agent_id}-${item.container_id}` }))
        this.containersTotal = (res.data && res.data.total) || 0
        this.containersPagination.total = this.containersTotal
      } catch (error) {
        console.error('获取容器资产失败:', error)
        this.$message.error('获取容器资产失败')
      } finally {
        this.containersLoading = false
      }
    },

    handlePortsTableChange(pagination) {
      this.portsPagination = pagination
      this.fetchPorts()
    },
    handleSoftwareTableChange(pagination) {
      this.softwarePagination = pagination
      this.fetchSoftware()
    },
    handleContainersTableChange(pagination) {
      this.containersPagination = pagination
      this.fetchContainers()
    },

    handleHostChange() {
      this.fetchPorts()
      this.fetchSoftware()
      this.fetchContainers()
    },
    handleScan() {
      this.$message.success('正在执行资产扫描...')
    },
    handleRefresh() {
      this.fetchHosts()
      this.fetchPorts()
      this.fetchSoftware()
      this.fetchContainers()
    },
    handleExport() {
      this.$message.info('导出资产报告')
    },
    handleServicesSearch() {
      this.$message.info(`搜索服务: ${this.servicesSearch}`)
    },
    handleJavaSearch() {
      this.$message.info(`搜索Java应用: ${this.javaSearch}`)
    },
    handleDepsSearch() {
      this.$message.info(`搜索依赖库: ${this.depsSearch}`)
    },
    handleDepTypeChange() {
      this.$message.info(`切换到: ${this.depType}`)
    },
    getDepTypeColor(type) {
      const colors = { Python: 'blue', 'Node.js': 'green', Java: 'red', Go: 'cyan' }
      return colors[type] || 'default'
    }
  }
}
</script>

<style lang="less" scoped>
.asset-header {
  margin-bottom: 24px;
}

.asset-stats {
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
        font-size: 28px;
        font-weight: 600;
        line-height: 1;
        margin-bottom: 4px;
        color: #262626;
      }

      .stat-label {
        font-size: 14px;
        color: #8c8c8c;
      }
    }
  }
}
</style>
