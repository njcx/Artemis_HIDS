<template>
  <div class="artemis-login">
    <div class="login-left">
      <div class="particles"></div>
      <div class="brand-info">
        <div class="logo-section">
          <div class="logo-icon">
            <a-icon type="safety-certificate" />
          </div>
          <h1 class="brand-title">Artemis HIDS</h1>
        </div>
        <p class="brand-subtitle">Host Intrusion Detection System</p>
        <div class="feature-list">
          <div class="feature-item">
            <a-icon type="check-circle" />
            <span>实时监控主机行为</span>
          </div>
          <div class="feature-item">
            <a-icon type="check-circle" />
            <span>智能威胁检测分析</span>
          </div>
          <div class="feature-item">
            <a-icon type="check-circle" />
            <span>快速安全事件响应</span>
          </div>
          <div class="feature-item">
            <a-icon type="check-circle" />
            <span>全方位安全防护</span>
          </div>
        </div>
      </div>
    </div>
    
    <div class="login-right">
      <div class="login-container">
        <div class="login-card">
          <div class="login-header">
            <h2>欢迎回来</h2>
            <p>登录您的 Artemis HIDS 账户</p>
          </div>
          
          <a-form-model ref="loginForm" :model="form" :rules="rules" @submit.native.prevent="handleLogin">
            <a-form-model-item prop="username">
              <div class="input-label">用户名</div>
              <a-input 
                v-model="form.username" 
                size="large" 
                placeholder="请输入用户名"
                autocomplete="username"
              >
                <a-icon slot="prefix" type="user" />
              </a-input>
            </a-form-model-item>
            
            <a-form-model-item prop="password">
              <div class="input-label">密码</div>
              <a-input-password 
                v-model="form.password" 
                size="large" 
                placeholder="请输入密码"
                autocomplete="current-password"
              >
                <a-icon slot="prefix" type="lock" />
              </a-input-password>
            </a-form-model-item>
            
            <a-form-model-item prop="captcha">
              <div class="input-label">验证码</div>
              <div class="captcha-container">
                <a-input 
                  v-model="form.captcha" 
                  size="large" 
                  placeholder="请输入验证码"
                  style="width: calc(100% - 140px);"
                >
                  <a-icon slot="prefix" type="safety" />
                </a-input>
                <div class="captcha-image-wrapper" @click="refreshCaptcha">
                  <img 
                    v-if="captchaImage" 
                    :src="captchaImage" 
                    alt="验证码"
                    class="captcha-image"
                  />
                  <div v-else class="captcha-loading">
                    <a-spin size="small" />
                  </div>
                  <div class="captcha-refresh-icon">
                    <a-icon type="reload" />
                  </div>
                </div>
              </div>
            </a-form-model-item>
            
            <a-form-model-item style="margin-bottom: 12px;">
              <a-checkbox>记住我</a-checkbox>
            </a-form-model-item>
            
            <a-form-model-item style="margin-bottom: 0;">
              <a-button 
                type="primary" 
                html-type="submit" 
                size="large" 
                :loading="logging"
                block
              >
                <span v-if="!logging">登录系统</span>
                <span v-else>登录中...</span>
              </a-button>
            </a-form-model-item>
          </a-form-model>
          
          <div class="login-footer">
            <div class="footer-text">
              <a-icon type="info-circle" />
              <span>需要帮助？请联系系统管理员</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios'

export default {
  name: 'Login',
  data() {
    return {
      logging: false,
      captchaImage: '',
      captchaID: '',
      captchaTimestamp: 0,
      form: {
        username: '',
        password: '',
        captcha: ''
      },
      rules: {
        username: [
          { required: true, message: '请输入用户名', trigger: 'blur' }
        ],
        password: [
          { required: true, message: '请输入密码', trigger: 'blur' }
        ],
        captcha: [
          { required: true, message: '请输入验证码', trigger: 'blur' }
        ]
      }
    }
  },
  mounted() {
    this.loadCaptcha()
  },
  methods: {
    // 加载验证码
    loadCaptcha() {
      // 生成唯一的时间戳
      this.captchaTimestamp = new Date().getTime()
      
      // 使用 fetch 获取验证码
      fetch('/api/v1/auth/captcha?t=' + this.captchaTimestamp)
        .then(response => {
          // 获取验证码ID
          this.captchaID = response.headers.get('x-captcha-id') || response.headers.get('X-Captcha-ID')
          console.log('验证码ID:', this.captchaID)
          
          // 将响应转为blob
          return response.blob()
        })
        .then(blob => {
          // 创建对象URL
          this.captchaImage = URL.createObjectURL(blob)
        })
        .catch(error => {
          console.error('验证码加载失败:', error)
          this.$message.error('验证码加载失败')
        })
    },
    
    // 刷新验证码
    refreshCaptcha() {
      this.captchaImage = ''
      this.form.captcha = ''
      this.loadCaptcha()
    },
    
    handleLogin() {
      this.$refs.loginForm.validate(valid => {
        if (valid) {
          this.logging = true
          
          axios.post('/api/v1/auth/login', {
            username: this.form.username,
            password: this.form.password,
            captcha: this.form.captcha,
            captcha_id: this.captchaID
          })
          .then(response => {
            const { token, expires_at, user } = response.data
            
            // 存储 token 到 localStorage
            localStorage.setItem('token', token)
            localStorage.setItem('token_expires_at', expires_at)
            
            // 存储用户信息到 localStorage (使用环境变量定义的 key)
            const userData = {
              id: user.id,
              name: user.full_name || user.username,
              avatar: user.avatar || '',
              ...user
            }
            localStorage.setItem(process.env.VUE_APP_USER_KEY, JSON.stringify(userData))
            
            this.$message.success(`欢迎回来，${user.full_name || user.username}！`)
            
            // 跳转到仪表板
            this.$router.push('/dashboard/artemis')
          })
          .catch(error => {
            this.$message.error(error.response?.data?.message || '登录失败，请检查用户名和密码')
            // 刷新验证码
            this.refreshCaptcha()
          })
          .finally(() => {
            this.logging = false
          })
        }
      })
    }
  }
}
</script>

<style lang="less" scoped>
.artemis-login {
  min-height: 100vh;
  display: flex;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
  
  .login-left {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 80px 100px;
    position: relative;
    
    .particles {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-image: 
        radial-gradient(circle at 15% 20%, rgba(255, 255, 255, 0.1) 0%, transparent 40%),
        radial-gradient(circle at 85% 80%, rgba(255, 255, 255, 0.08) 0%, transparent 40%),
        radial-gradient(circle at 50% 50%, rgba(255, 255, 255, 0.05) 0%, transparent 50%);
      animation: float 15s ease-in-out infinite;
    }
    
    @keyframes float {
      0%, 100% {
        transform: translateY(0) scale(1);
        opacity: 0.8;
      }
      50% {
        transform: translateY(-20px) scale(1.05);
        opacity: 1;
      }
    }
    
    .brand-info {
      max-width: 650px;
      color: #fff;
      position: relative;
      z-index: 1;
      
      .logo-section {
        display: flex;
        align-items: center;
        margin-bottom: 32px;
        
        .logo-icon {
          width: 90px;
          height: 90px;
          background: rgba(255, 255, 255, 0.15);
          backdrop-filter: blur(10px);
          border-radius: 20px;
          display: flex;
          align-items: center;
          justify-content: center;
          margin-right: 24px;
          box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
          
          .anticon {
            font-size: 48px;
            color: #fff;
          }
        }
        
        .brand-title {
          font-size: 68px;
          font-weight: 900;
          margin: 0;
          letter-spacing: 2px;
          text-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
          line-height: 1;
        }
      }
      
      .brand-subtitle {
        font-size: 20px;
        font-weight: 300;
        margin-bottom: 60px;
        opacity: 0.9;
        letter-spacing: 3px;
        text-transform: uppercase;
        padding-left: 114px;
      }
      
      .feature-list {
        padding-left: 114px;
        
        .feature-item {
          display: flex;
          align-items: center;
          font-size: 18px;
          margin-bottom: 20px;
          opacity: 0.95;
          transition: all 0.3s ease;
          
          &:hover {
            opacity: 1;
            transform: translateX(10px);
          }
          
          .anticon {
            font-size: 22px;
            margin-right: 16px;
            color: #fff;
          }
          
          span {
            font-weight: 400;
          }
        }
      }
    }
  }
  
  .login-right {
    width: 520px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #fff;
    box-shadow: -20px 0 60px rgba(0, 0, 0, 0.15);
    position: relative;
    z-index: 2;
    
    .login-container {
      width: 100%;
      padding: 80px 70px;
      
      .login-card {
        .login-header {
          text-align: center;
          margin-bottom: 48px;
          
          h2 {
            font-size: 36px;
            font-weight: 700;
            color: #1a1a1a;
            margin-bottom: 12px;
            letter-spacing: -0.5px;
          }
          
          p {
            font-size: 15px;
            color: #8c8c8c;
            margin: 0;
          }
        }
        
        .input-label {
          font-size: 14px;
          font-weight: 600;
          color: #262626;
          margin-bottom: 8px;
          letter-spacing: 0.3px;
        }
        
        /deep/ .ant-form-item {
          margin-bottom: 24px;
        }
        
        /deep/ .ant-input-affix-wrapper,
        /deep/ .ant-input {
          height: 52px;
          border-radius: 10px;
          font-size: 15px;
          border: 2px solid #f0f0f0;
          background: #fafafa;
          transition: all 0.3s ease;
          
          &:hover {
            background: #fff;
            border-color: #d9d9d9;
          }
          
          &:focus,
          &.ant-input-affix-wrapper-focused {
            background: #fff;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.08);
          }
        }
        
        /deep/ .ant-input::placeholder {
          color: #bfbfbf;
        }
        
        /deep/ .anticon {
          color: #bfbfbf;
          font-size: 16px;
        }
        
        /deep/ .ant-checkbox-wrapper {
          color: #595959;
          font-size: 14px;
          
          .ant-checkbox-checked .ant-checkbox-inner {
            background-color: #667eea;
            border-color: #667eea;
          }
        }
        
        /deep/ .ant-btn-primary {
          height: 52px;
          font-size: 16px;
          font-weight: 600;
          border-radius: 10px;
          background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
          border: none;
          margin-top: 12px;
          letter-spacing: 1px;
          box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
          transition: all 0.3s ease;
          
          &:hover,
          &:focus {
            transform: translateY(-2px);
            box-shadow: 0 8px 25px rgba(102, 126, 234, 0.4);
          }
          
          &:active {
            transform: translateY(0);
          }
        }
        
        .captcha-container {
          display: flex;
          gap: 10px;
          align-items: center;
          
          .captcha-image-wrapper {
            width: 120px;
            height: 52px;
            border-radius: 10px;
            overflow: hidden;
            cursor: pointer;
            position: relative;
            border: 2px solid #f0f0f0;
            background: #fafafa;
            transition: all 0.3s ease;
            
            &:hover {
              border-color: #667eea;
              box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.08);
              
              .captcha-refresh-icon {
                opacity: 1;
              }
            }
            
            .captcha-image {
              width: 100%;
              height: 100%;
              object-fit: cover;
              display: block;
            }
            
            .captcha-loading {
              width: 100%;
              height: 100%;
              display: flex;
              align-items: center;
              justify-content: center;
            }
            
            .captcha-refresh-icon {
              position: absolute;
              top: 0;
              left: 0;
              right: 0;
              bottom: 0;
              background: rgba(0, 0, 0, 0.5);
              display: flex;
              align-items: center;
              justify-content: center;
              opacity: 0;
              transition: opacity 0.3s ease;
              
              .anticon {
                color: #fff;
                font-size: 20px;
              }
            }
          }
        }
        
        .login-footer {
          margin-top: 32px;
          padding-top: 24px;
          border-top: 1px solid #f0f0f0;
          
          .footer-text {
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 13px;
            color: #8c8c8c;
            
            .anticon {
              margin-right: 8px;
              font-size: 14px;
            }
          }
        }
      }
    }
  }
  
  @media screen and (max-width: 1200px) {
    .login-left {
      padding: 60px 60px;
      
      .brand-info {
        .logo-section {
          .logo-icon {
            width: 70px;
            height: 70px;
            
            .anticon {
              font-size: 36px;
            }
          }
          
          .brand-title {
            font-size: 52px;
          }
        }
        
        .brand-subtitle {
          font-size: 16px;
          padding-left: 94px;
        }
        
        .feature-list {
          padding-left: 94px;
          
          .feature-item {
            font-size: 16px;
          }
        }
      }
    }
  }
  
  @media screen and (max-width: 1024px) {
    .login-left {
      display: none;
    }
    
    .login-right {
      width: 100%;
    }
  }
  
  @media screen and (max-width: 576px) {
    .login-right {
      .login-container {
        padding: 40px 30px;
      }
    }
  }
}
</style>
