<template>
  <div class="artemis-login">
    <div class="login-left">
      <div class="grid-pattern"></div>
      <div class="brand-info">
        <div class="logo-section">
          <div class="logo-icon">
            <a-icon type="safety-certificate" />
          </div>
          <span class="brand-name">Artemis</span>
        </div>

        <h1 class="brand-title">主机入侵检测系统</h1>
        <p class="brand-desc">
          实时监控主机行为、检测入侵威胁与异常事件，<br />
          为您的基础设施筑起企业级安全防线。
        </p>

        <div class="feature-grid">
          <div class="feature-card">
            <span class="feature-icon"><a-icon type="thunderbolt" /></span>
            <span>实时事件采集</span>
          </div>
          <div class="feature-card">
            <span class="feature-icon"><a-icon type="radar-chart" /></span>
            <span>多维度检测</span>
          </div>
          <div class="feature-card">
            <span class="feature-icon"><a-icon type="bar-chart" /></span>
            <span>可视化分析</span>
          </div>
          <div class="feature-card">
            <span class="feature-icon"><a-icon type="lock" /></span>
            <span>高性能规则引擎</span>
          </div>
        </div>
      </div>
    </div>

    <div class="login-right">
      <div class="login-container">
        <div class="login-card">
          <div class="login-header">
            <h2>欢迎回来 <span class="wave">👋</span></h2>
            <p>登录 Artemis HIDS 后台管理系统</p>
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

            <a-form-model-item style="margin-bottom: 0;">
              <a-button
                type="primary"
                html-type="submit"
                size="large"
                :loading="logging"
                block
              >
                <span v-if="!logging">登 录</span>
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
  background: #0f2557;
  position: relative;
  overflow: hidden;

  .login-left {
    flex: 1;
    display: flex;
    align-items: center;
    padding: 0 100px;
    position: relative;
    background: linear-gradient(135deg, #1a3a7a 0%, #2255b0 55%, #3f7fd6 100%);

    .grid-pattern {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-image:
        linear-gradient(rgba(255, 255, 255, 0.06) 1px, transparent 1px),
        linear-gradient(90deg, rgba(255, 255, 255, 0.06) 1px, transparent 1px);
      background-size: 36px 36px;
      mask-image: radial-gradient(circle at 30% 40%, rgba(0, 0, 0, 0.9) 0%, transparent 65%);
    }

    .brand-info {
      max-width: 560px;
      color: #fff;
      position: relative;
      z-index: 1;

      .logo-section {
        display: flex;
        align-items: center;
        margin-bottom: 40px;

        .logo-icon {
          width: 40px;
          height: 40px;
          background: rgba(255, 255, 255, 0.18);
          border-radius: 10px;
          display: flex;
          align-items: center;
          justify-content: center;
          margin-right: 12px;

          .anticon {
            font-size: 22px;
            color: #fff;
          }
        }

        .brand-name {
          font-size: 22px;
          font-weight: 600;
          letter-spacing: 0.5px;
        }
      }

      .brand-title {
        font-size: 42px;
        font-weight: 700;
        margin: 0 0 20px;
        line-height: 1.2;
      }

      .brand-desc {
        font-size: 15px;
        line-height: 1.8;
        opacity: 0.85;
        margin-bottom: 40px;
      }

      .feature-grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 14px;
        max-width: 420px;

        .feature-card {
          display: flex;
          align-items: center;
          gap: 10px;
          padding: 14px 16px;
          background: rgba(255, 255, 255, 0.08);
          border: 1px solid rgba(255, 255, 255, 0.12);
          border-radius: 10px;
          font-size: 14px;
          transition: background 0.3s ease;

          &:hover {
            background: rgba(255, 255, 255, 0.14);
          }

          .feature-icon {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 30px;
            height: 30px;
            border-radius: 8px;
            background: rgba(255, 255, 255, 0.12);

            .anticon {
              font-size: 15px;
              color: #fff;
            }
          }
        }
      }
    }
  }

  .login-right {
    width: 480px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #fff;
    position: relative;
    z-index: 2;

    .login-container {
      width: 100%;
      padding: 0 60px;

      .login-card {
        .login-header {
          margin-bottom: 40px;

          h2 {
            font-size: 26px;
            font-weight: 700;
            color: #1a1a1a;
            margin-bottom: 8px;

            .wave {
              display: inline-block;
            }
          }

          p {
            font-size: 14px;
            color: #8c8c8c;
            margin: 0;
          }
        }

        .input-label {
          font-size: 14px;
          color: #595959;
          margin-bottom: 8px;
        }

        /deep/ .ant-form-item {
          margin-bottom: 20px;
        }

        /deep/ .ant-input-affix-wrapper,
        /deep/ .ant-input {
          height: 44px;
          border-radius: 8px;
          font-size: 14px;
          border: 1px solid #e2e5ea;
          background: #fff;
          transition: all 0.3s ease;

          &:hover {
            border-color: #a9bdda;
          }

          &:focus,
          &.ant-input-affix-wrapper-focused {
            border-color: #2255b0;
            box-shadow: 0 0 0 2px rgba(34, 85, 176, 0.1);
          }
        }

        /deep/ .ant-input::placeholder {
          color: #bfbfbf;
        }

        /deep/ .anticon {
          color: #bfbfbf;
          font-size: 15px;
        }

        /deep/ .ant-btn-primary {
          height: 44px;
          font-size: 15px;
          font-weight: 600;
          border-radius: 8px;
          background: linear-gradient(135deg, #2255b0 0%, #16386f 100%);
          border: none;
          margin-top: 8px;
          transition: all 0.3s ease;

          &:hover,
          &:focus {
            opacity: 0.92;
          }
        }

        .captcha-container {
          display: flex;
          gap: 10px;
          align-items: center;

          .captcha-image-wrapper {
            width: 120px;
            height: 44px;
            border-radius: 8px;
            overflow: hidden;
            cursor: pointer;
            position: relative;
            border: 1px solid #e2e5ea;
            background: #fafafa;
            transition: all 0.3s ease;

            &:hover {
              border-color: #2255b0;

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
                font-size: 18px;
              }
            }
          }
        }

        .login-footer {
          margin-top: 28px;
          text-align: center;

          .footer-text {
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 13px;
            color: #8c8c8c;

            .anticon {
              margin-right: 6px;
              font-size: 13px;
            }
          }
        }
      }
    }
  }

  @media screen and (max-width: 1200px) {
    .login-left {
      padding: 0 60px;

      .brand-info {
        .brand-title {
          font-size: 34px;
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
        padding: 0 30px;
      }
    }
  }
}
</style>
