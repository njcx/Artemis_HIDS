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
                <a-icon type="apartment" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.services }}</div>
                <div class="stat-label">服务</div>
              </div>
            </div>
          </a-col>
          <a-col :span="6">
            <div class="stat-card">
              <div class="stat-icon" style="background: linear-gradient(135deg, #52c41a 0%, #73d13d 100%);">
                <a-icon type="api" />
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ stats.ports }}</div>
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
                <div class="stat-value">{{ stats.packages }}</div>
                <div class="stat-label">软件包</div>
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
            placeholder="搜索端口或服务"
            style="width: 300px; margin-bottom: 16px"
            @search="handlePortsSearch"
          />
          <a-table
            :columns="portColumns"
            :data-source="portsData"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="id"
          >
            <template slot="port" slot-scope="port, record">
              <a-tag color="blue">{{ port }}/{{ record.protocol }}</a-tag>
            </template>
            
            <template slot="state" slot-scope="state">
              <a-badge :status="state === 'LISTEN' ? 'success' : 'default'" :text="state" />
            </template>
            
            <template slot="risk" slot-scope="risk">
              <a-tag :color="getRiskColor(risk)">
                {{ getRiskText(risk) }}
              </a-tag>
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
            <a-radio-group v-model="packageType" button-style="solid" @change="handlePackageTypeChange">
              <a-radio-button value="all">全部</a-radio-button>
              <a-radio-button value="yum">YUM/RPM</a-radio-button>
              <a-radio-button value="apt">APT/DEB</a-radio-button>
              <a-radio-button value="pip">PIP</a-radio-button>
              <a-radio-button value="npm">NPM</a-radio-button>
            </a-radio-group>
            <a-input-search
              v-model="packagesSearch"
              placeholder="搜索软件包名称"
              style="width: 300px; margin-left: 16px"
              @search="handlePackagesSearch"
            />
          </div>
          <a-table
            :columns="packageColumns"
            :data-source="packagesData"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="id"
          >
            <template slot="type" slot-scope="type">
              <a-tag :color="getPackageTypeColor(type)">
                {{ type.toUpperCase() }}
              </a-tag>
            </template>
            
            <template slot="hasUpdate" slot-scope="hasUpdate, record">
              <span v-if="hasUpdate">
                <a-tag color="orange">可更新</a-tag>
                <span style="color: #8c8c8c">{{ record.latestVersion }}</span>
              </span>
              <span v-else style="color: #52c41a">
                <a-icon type="check-circle" /> 最新
              </span>
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

export default {
  name: 'AssetDiscovery',
  components: { PageLayout },
  data() {
    return {
      selectedHost: 'all',
      hosts: [
        { id: 1, hostname: 'web-server-01', ip: '192.168.1.101' },
        { id: 2, hostname: 'db-server-02', ip: '192.168.1.102' },
        { id: 3, hostname: 'app-server-03', ip: '192.168.1.103' }
      ],
      stats: {
        services: 45,
        ports: 23,
        packages: 1247,
        dependencies: 328
      },
      activeTab: 'ports',
      loading: false,
      portsSearch: '',
      servicesSearch: '',
      packagesSearch: '',
      javaSearch: '',
      depsSearch: '',
      packageType: 'all',
      depType: 'all',
      
      // 端口数据
      portsData: [
        { id: 1, hostname: 'web-server-01', port: 22, protocol: 'TCP', service: 'SSH', process: 'sshd', state: 'LISTEN', risk: 'low' },
        { id: 2, hostname: 'web-server-01', port: 80, protocol: 'TCP', service: 'HTTP', process: 'nginx', state: 'LISTEN', risk: 'medium' },
        { id: 3, hostname: 'web-server-01', port: 443, protocol: 'TCP', service: 'HTTPS', process: 'nginx', state: 'LISTEN', risk: 'low' },
        { id: 4, hostname: 'db-server-02', port: 3306, protocol: 'TCP', service: 'MySQL', process: 'mysqld', state: 'LISTEN', risk: 'high' },
        { id: 5, hostname: 'app-server-03', port: 8080, protocol: 'TCP', service: 'HTTP-Alt', process: 'java', state: 'LISTEN', risk: 'medium' }
      ],
      portColumns: [
        { title: '主机', dataIndex: 'hostname', key: 'hostname', width: 150 },
        { title: '端口', dataIndex: 'port', key: 'port', width: 100, scopedSlots: { customRender: 'port' } },
        { title: '服务', dataIndex: 'service', key: 'service', width: 120 },
        { title: '进程', dataIndex: 'process', key: 'process', width: 120 },
        { title: '状态', dataIndex: 'state', key: 'state', width: 100, scopedSlots: { customRender: 'state' } },
        { title: '风险级别', dataIndex: 'risk', key: 'risk', width: 100, scopedSlots: { customRender: 'risk' } }
      ],
      
      // 服务数据
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
      
      // 软件包数据
      packagesData: [
        { id: 1, hostname: 'web-server-01', name: 'nginx', version: '1.18.0', type: 'yum', hasUpdate: false, latestVersion: null, installDate: '2024-01-15' },
        { id: 2, hostname: 'web-server-01', name: 'python3', version: '3.8.10', type: 'apt', hasUpdate: true, latestVersion: '3.8.12', installDate: '2024-02-20' },
        { id: 3, hostname: 'db-server-02', name: 'mysql-server', version: '8.0.26', type: 'apt', hasUpdate: false, latestVersion: null, installDate: '2024-03-10' },
        { id: 4, hostname: 'app-server-03', name: 'requests', version: '2.25.1', type: 'pip', hasUpdate: true, latestVersion: '2.28.0', installDate: '2024-04-05' },
        { id: 5, hostname: 'app-server-03', name: 'express', version: '4.17.1', type: 'npm', hasUpdate: true, latestVersion: '4.18.2', installDate: '2024-05-12' }
      ],
      packageColumns: [
        { title: '主机', dataIndex: 'hostname', key: 'hostname', width: 150 },
        { title: '包名', dataIndex: 'name', key: 'name', width: 180 },
        { title: '版本', dataIndex: 'version', key: 'version', width: 120 },
        { title: '类型', dataIndex: 'type', key: 'type', width: 100, scopedSlots: { customRender: 'type' } },
        { title: '安装日期', dataIndex: 'installDate', key: 'installDate', width: 120 },
        { title: '更新状态', key: 'hasUpdate', width: 150, scopedSlots: { customRender: 'hasUpdate' } }
      ],
      
      // Java应用数据
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
      
      // 依赖库数据
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
  methods: {
    handleHostChange(value) {
      this.$message.info(`切换到主机: ${value}`)
      this.loading = true
      setTimeout(() => {
        this.loading = false
      }, 500)
    },
    handleScan() {
      this.$message.success('正在执行资产扫描...')
    },
    handleRefresh() {
      this.loading = true
      setTimeout(() => {
        this.loading = false
        this.$message.success('刷新成功')
      }, 500)
    },
    handleExport() {
      this.$message.info('导出资产报告')
    },
    handlePortsSearch() {
      this.$message.info(`搜索端口: ${this.portsSearch}`)
    },
    handleServicesSearch() {
      this.$message.info(`搜索服务: ${this.servicesSearch}`)
    },
    handlePackagesSearch() {
      this.$message.info(`搜索软件包: ${this.packagesSearch}`)
    },
    handleJavaSearch() {
      this.$message.info(`搜索Java应用: ${this.javaSearch}`)
    },
    handleDepsSearch() {
      this.$message.info(`搜索依赖库: ${this.depsSearch}`)
    },
    handlePackageTypeChange() {
      this.$message.info(`切换到: ${this.packageType}`)
    },
    handleDepTypeChange() {
      this.$message.info(`切换到: ${this.depType}`)
    },
    getRiskColor(risk) {
      const colors = { low: 'green', medium: 'orange', high: 'red' }
      return colors[risk] || 'default'
    },
    getRiskText(risk) {
      const texts = { low: '低风险', medium: '中风险', high: '高风险' }
      return texts[risk] || risk
    },
    getPackageTypeColor(type) {
      const colors = { yum: 'red', apt: 'purple', pip: 'blue', npm: 'green' }
      return colors[type] || 'default'
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
