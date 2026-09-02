import {removeAuthorization} from '@/utils/request'

// 401拦截：token 缺失/过期/失效，清除本地登录态并强制跳转登录页
const resp401 = {
  /**
   * 响应数据之前做点什么
   * @param response 响应对象
   * @param options 应用配置 包含: {router, i18n, store, message}
   * @returns {*}
   */
  onFulfilled(response, options) {
    const {message} = options
    if (response.code === 401) {
      message.error('无此权限')
    }
    return response
  },
  /**
   * 响应出错时执行
   * @param error 错误对象
   * @param options 应用配置 包含: {router, i18n, store, message}
   * @returns {Promise<never>}
   */
  onRejected(error, options) {
    const {message, router} = options
    const {response} = error
    if (response.status === 401) {
      removeAuthorization()
      if (router && router.currentRoute.path !== '/login') {
        message.error('登录已失效，请重新登录')
        router.push('/login')
      }
    }
    return Promise.reject(error)
  }
}

const resp403 = {
  onFulfilled(response, options) {
    const {message} = options
    if (response.code === 403) {
      message.error('请求被拒绝')
    }
    return response
  },
  onRejected(error, options) {
    const {message} = options
    const {response} = error
    if (response.status === 403) {
      message.error('请求被拒绝')
    }
    return Promise.reject(error)
  }
}

const reqCommon = {
  /**
   * 发送请求之前做些什么
   * @param config axios config
   * @param options 应用配置 包含: {router, i18n, store, message}
   * @returns {*}
   */
  onFulfilled(config, options) {
    const {message} = options
    const {url} = config
    
    // 如果不是登录接口，检查并添加 token
    if (url.indexOf('login') === -1) {
      const token = localStorage.getItem('token')
      if (!token || token === 'null' || token === 'undefined') {
        message.warning('认证 token 已过期，请重新登录')
      } else {
        // 添加 Authorization 头
        config.headers['Authorization'] = `Bearer ${token}`
      }
    }
    return config
  },
  /**
   * 请求出错时做点什么
   * @param error 错误对象
   * @param options 应用配置 包含: {router, i18n, store, message}
   * @returns {Promise<never>}
   */
  onRejected(error, options) {
    const {message} = options
    message.error(error.message)
    return Promise.reject(error)
  }
}

export default {
  request: [reqCommon], // 请求拦截
  response: [resp401, resp403] // 响应拦截
}
