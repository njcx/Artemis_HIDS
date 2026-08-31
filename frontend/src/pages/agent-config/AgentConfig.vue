<template>
  <page-layout>
    <a-card :bordered="false">
      <a-tabs v-model="activeTab">
        <!-- 全局配置 -->
        <a-tab-pane key="global" tab="全局配置">
          <a-form-model
            ref="globalForm"
            :model="globalConfig"
            :label-col="{ span: 4 }"
            :wrapper-col="{ span: 16 }"
          >
            <a-divider orientation="left">基础配置</a-divider>
            
            <a-form-model-item label="Agent版本">
              <a-input v-model="globalConfig.version" disabled />
            </a-form-model-item>
            
            <a-form-model-item label="安装脚本地址">
              <a-input v-model="globalConfig.installScriptUrl" placeholder="http://localhost:8080/download/install.sh">
                <a-icon slot="prefix" type="link" />
              </a-input>
              <div class="script-actions" style="margin-top: 8px;">
                <a-button size="small" icon="copy" @click="handleCopyScript">
                  复制安装命令
                </a-button>
                <a-button size="small" icon="download" style="margin-left: 8px" @click="handleDownloadScript">
                  下载脚本
                </a-button>
              </div>
              <span class="form-help">Agent安装脚本的下载地址，用于快速部署</span>
            </a-form-model-item>
            
            <a-form-model-item label="服务器地址">
              <a-input v-model="globalConfig.serverUrl" placeholder="http://localhost:8080" />
              <span class="form-help">Manager服务器地址</span>
            </a-form-model-item>
            
            <a-form-model-item label="心跳间隔">
              <a-input-number v-model="globalConfig.heartbeatInterval" :min="10" :max="300" style="width: 200px" />
              <span style="margin-left: 8px">秒</span>
            </a-form-model-item>
            
            <a-form-model-item label="数据上报间隔">
              <a-input-number v-model="globalConfig.reportInterval" :min="30" :max="600" style="width: 200px" />
              <span style="margin-left: 8px">秒</span>
            </a-form-model-item>
            
            <a-divider orientation="left">日志配置</a-divider>
            
            <a-form-model-item label="日志级别">
              <a-select v-model="globalConfig.logLevel" style="width: 200px">
                <a-select-option value="debug">DEBUG</a-select-option>
                <a-select-option value="info">INFO</a-select-option>
                <a-select-option value="warn">WARN</a-select-option>
                <a-select-option value="error">ERROR</a-select-option>
              </a-select>
            </a-form-model-item>
            
            <a-form-model-item label="日志文件路径">
              <a-input v-model="globalConfig.logPath" placeholder="/var/log/artemis-agent" />
            </a-form-model-item>
            
            <a-form-model-item label="日志文件大小">
              <a-input-number v-model="globalConfig.logMaxSize" :min="10" :max="1000" style="width: 200px" />
              <span style="margin-left: 8px">MB</span>
            </a-form-model-item>
            
            <a-form-model-item label="日志保留天数">
              <a-input-number v-model="globalConfig.logRetentionDays" :min="1" :max="365" style="width: 200px" />
              <span style="margin-left: 8px">天</span>
            </a-form-model-item>
            
            <a-form-model-item :wrapper-col="{ span: 16, offset: 4 }">
              <a-button type="primary" @click="handleSaveGlobal">
                保存配置
              </a-button>
              <a-button style="margin-left: 8px" @click="handleResetGlobal">
                重置
              </a-button>
            </a-form-model-item>
          </a-form-model>
        </a-tab-pane>

        <!-- 监控配置 -->
        <a-tab-pane key="monitor" tab="监控配置">
          <a-form-model
            ref="monitorForm"
            :model="monitorConfig"
            :label-col="{ span: 4 }"
            :wrapper-col="{ span: 16 }"
          >
            <a-divider orientation="left">文件监控</a-divider>
            
            <a-form-model-item label="启用文件监控">
              <a-switch v-model="monitorConfig.fileMonitor.enabled" />
            </a-form-model-item>
            
            <a-form-model-item label="监控路径" v-if="monitorConfig.fileMonitor.enabled">
              <a-select
                v-model="monitorConfig.fileMonitor.paths"
                mode="tags"
                style="width: 100%"
                placeholder="输入路径后按回车添加"
              />
              <span class="form-help">支持多个路径，使用回车添加</span>
            </a-form-model-item>
            
            <a-form-model-item label="排除路径" v-if="monitorConfig.fileMonitor.enabled">
              <a-select
                v-model="monitorConfig.fileMonitor.excludePaths"
                mode="tags"
                style="width: 100%"
                placeholder="输入路径后按回车添加"
              />
            </a-form-model-item>
            
            <a-divider orientation="left">进程监控</a-divider>
            
            <a-form-model-item label="启用进程监控">
              <a-switch v-model="monitorConfig.processMonitor.enabled" />
            </a-form-model-item>
            
            <a-form-model-item label="扫描间隔" v-if="monitorConfig.processMonitor.enabled">
              <a-input-number v-model="monitorConfig.processMonitor.scanInterval" :min="5" :max="300" style="width: 200px" />
              <span style="margin-left: 8px">秒</span>
            </a-form-model-item>
            
            <a-form-model-item label="监控用户进程" v-if="monitorConfig.processMonitor.enabled">
              <a-switch v-model="monitorConfig.processMonitor.monitorUserProcesses" />
            </a-form-model-item>
            
            <a-divider orientation="left">网络监控</a-divider>
            
            <a-form-model-item label="启用网络监控">
              <a-switch v-model="monitorConfig.networkMonitor.enabled" />
            </a-form-model-item>
            
            <a-form-model-item label="监控端口" v-if="monitorConfig.networkMonitor.enabled">
              <a-select
                v-model="monitorConfig.networkMonitor.ports"
                mode="tags"
                style="width: 100%"
                placeholder="输入端口号后按回车添加"
              />
            </a-form-model-item>
            
            <a-form-model-item label="监控协议" v-if="monitorConfig.networkMonitor.enabled">
              <a-checkbox-group v-model="monitorConfig.networkMonitor.protocols">
                <a-checkbox value="tcp">TCP</a-checkbox>
                <a-checkbox value="udp">UDP</a-checkbox>
                <a-checkbox value="icmp">ICMP</a-checkbox>
              </a-checkbox-group>
            </a-form-model-item>
            
            <a-form-model-item :wrapper-col="{ span: 16, offset: 4 }">
              <a-button type="primary" @click="handleSaveMonitor">
                保存配置
              </a-button>
              <a-button style="margin-left: 8px" @click="handleResetMonitor">
                重置
              </a-button>
            </a-form-model-item>
          </a-form-model>
        </a-tab-pane>

        <!-- 性能配置 -->
        <a-tab-pane key="performance" tab="性能配置">
          <a-form-model
            ref="performanceForm"
            :model="performanceConfig"
            :label-col="{ span: 4 }"
            :wrapper-col="{ span: 16 }"
          >
            <a-divider orientation="left">资源限制</a-divider>
            
            <a-form-model-item label="CPU使用限制">
              <a-slider v-model="performanceConfig.cpuLimit" :min="10" :max="100" :marks="{ 10: '10%', 50: '50%', 100: '100%' }" />
            </a-form-model-item>
            
            <a-form-model-item label="内存使用限制">
              <a-input-number v-model="performanceConfig.memoryLimit" :min="128" :max="2048" style="width: 200px" />
              <span style="margin-left: 8px">MB</span>
            </a-form-model-item>
            
            <a-divider orientation="left">缓存配置</a-divider>
            
            <a-form-model-item label="事件缓存大小">
              <a-input-number v-model="performanceConfig.eventCacheSize" :min="100" :max="10000" style="width: 200px" />
              <span style="margin-left: 8px">条</span>
            </a-form-model-item>
            
            <a-form-model-item label="数据批量上报">
              <a-switch v-model="performanceConfig.batchUpload" />
              <span class="form-help" style="margin-left: 8px">启用后将批量上报数据以减少网络开销</span>
            </a-form-model-item>
            
            <a-form-model-item label="批量大小" v-if="performanceConfig.batchUpload">
              <a-input-number v-model="performanceConfig.batchSize" :min="10" :max="1000" style="width: 200px" />
              <span style="margin-left: 8px">条</span>
            </a-form-model-item>
            
            <a-divider orientation="left">工作线程</a-divider>
            
            <a-form-model-item label="工作线程数">
              <a-input-number v-model="performanceConfig.workerThreads" :min="1" :max="16" style="width: 200px" />
            </a-form-model-item>
            
            <a-form-model-item :wrapper-col="{ span: 16, offset: 4 }">
              <a-button type="primary" @click="handleSavePerformance">
                保存配置
              </a-button>
              <a-button style="margin-left: 8px" @click="handleResetPerformance">
                重置
              </a-button>
            </a-form-model-item>
          </a-form-model>
        </a-tab-pane>

        <!-- 配置模板 -->
        <a-tab-pane key="template" tab="配置模板">
          <div class="template-section">
            <a-alert
              message="配置模板"
              description="选择预设的配置模板快速应用到Agent，或者下载当前配置用于批量部署"
              type="info"
              show-icon
              style="margin-bottom: 24px"
            />
            
            <a-row :gutter="16">
              <a-col :span="8" v-for="template in templates" :key="template.id">
                <a-card :title="template.name" :bordered="false" class="template-card">
                  <p>{{ template.description }}</p>
                  <div class="template-actions">
                    <a-button type="primary" size="small" @click="handleApplyTemplate(template)">
                      应用模板
                    </a-button>
                    <a-button size="small" style="margin-left: 8px" @click="handleViewTemplate(template)">
                      查看详情
                    </a-button>
                  </div>
                </a-card>
              </a-col>
            </a-row>
            
            <a-divider />
            
            <div class="export-section">
              <h3>导出配置</h3>
              <a-space>
                <a-button icon="download" @click="handleExportJSON">
                  导出为 JSON
                </a-button>
                <a-button icon="download" @click="handleExportYAML">
                  导出为 YAML
                </a-button>
                <a-button icon="upload" @click="handleImport">
                  导入配置
                </a-button>
              </a-space>
            </div>
          </div>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </page-layout>
</template>

<script>
import PageLayout from '@/layouts/PageLayout'

export default {
  name: 'AgentConfig',
  components: { PageLayout },
  data() {
    return {
      activeTab: 'global',
      globalConfig: {
        version: '1.0.0',
        installScriptUrl: 'http://localhost:8080/download/install.sh',
        serverUrl: 'http://localhost:8080',
        heartbeatInterval: 30,
        reportInterval: 60,
        logLevel: 'info',
        logPath: '/var/log/artemis-agent',
        logMaxSize: 100,
        logRetentionDays: 30
      },
      monitorConfig: {
        fileMonitor: {
          enabled: true,
          paths: ['/etc', '/var/www', '/home'],
          excludePaths: ['/tmp', '/var/tmp']
        },
        processMonitor: {
          enabled: true,
          scanInterval: 30,
          monitorUserProcesses: true
        },
        networkMonitor: {
          enabled: true,
          ports: ['22', '80', '443', '3306'],
          protocols: ['tcp', 'udp']
        }
      },
      performanceConfig: {
        cpuLimit: 50,
        memoryLimit: 512,
        eventCacheSize: 1000,
        batchUpload: true,
        batchSize: 100,
        workerThreads: 4
      },
      templates: [
        {
          id: 1,
          name: '高安全级别',
          description: '适用于生产环境，启用所有监控功能，较低的上报间隔'
        },
        {
          id: 2,
          name: '标准级别',
          description: '适用于一般业务服务器，平衡性能和安全'
        },
        {
          id: 3,
          name: '低资源消耗',
          description: '适用于资源受限环境，最小化资源占用'
        }
      ]
    }
  },
  methods: {
    handleSaveGlobal() {
      this.$message.success('全局配置已保存')
    },
    handleResetGlobal() {
      this.$message.info('配置已重置')
    },
    handleSaveMonitor() {
      this.$message.success('监控配置已保存')
    },
    handleResetMonitor() {
      this.$message.info('配置已重置')
    },
    handleSavePerformance() {
      this.$message.success('性能配置已保存')
    },
    handleResetPerformance() {
      this.$message.info('配置已重置')
    },
    handleApplyTemplate(template) {
      this.$message.success(`已应用模板: ${template.name}`)
    },
    handleViewTemplate(template) {
      this.$message.info(`查看模板: ${template.name}`)
    },
    handleExportJSON() {
      this.$message.info('导出配置为 JSON 格式')
    },
    handleExportYAML() {
      this.$message.info('导出配置为 YAML 格式')
    },
    handleImport() {
      this.$message.info('导入配置')
    },
    handleCopyScript() {
      const command = `curl -fsSL ${this.globalConfig.installScriptUrl} | bash`
      
      // 复制到剪贴板
      if (navigator.clipboard) {
        navigator.clipboard.writeText(command).then(() => {
          this.$message.success('安装命令已复制到剪贴板')
        }).catch(() => {
          this.$message.error('复制失败，请手动复制')
        })
      } else {
        // 降级方案
        const textarea = document.createElement('textarea')
        textarea.value = command
        document.body.appendChild(textarea)
        textarea.select()
        try {
          document.execCommand('copy')
          this.$message.success('安装命令已复制到剪贴板')
        } catch (err) {
          this.$message.error('复制失败，请手动复制')
        }
        document.body.removeChild(textarea)
      }
    },
    handleDownloadScript() {
      window.open(this.globalConfig.installScriptUrl, '_blank')
      this.$message.info('正在下载安装脚本...')
    }
  }
}
</script>

<style lang="less" scoped>
.form-help {
  color: #8c8c8c;
  font-size: 12px;
}

.script-actions {
  display: inline-block;
}

.template-section {
  .template-card {
    margin-bottom: 16px;
    
    p {
      color: #595959;
      margin-bottom: 16px;
      min-height: 40px;
    }
    
    .template-actions {
      margin-top: 16px;
    }
  }
  
  .export-section {
    h3 {
      margin-bottom: 16px;
      font-size: 16px;
      font-weight: 600;
    }
  }
}
</style>
