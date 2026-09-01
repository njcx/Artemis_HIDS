<template>
  <a-dropdown>
    <div class="header-avatar" style="cursor: pointer">
      <a-avatar
        class="avatar"
        size="small"
        shape="circle"
        :src="user.avatar"
        :style="!user.avatar ? { backgroundColor: avatarColor, color: '#fff' } : {}"
      >
        <span v-if="!user.avatar">{{ avatarText }}</span>
      </a-avatar>
      <span class="name">{{user.name}}</span>
    </div>
    <a-menu :class="['avatar-menu']" slot="overlay">
      <!-- <a-menu-item>
        <a-icon type="user" />
        <span>个人中心</span>
      </a-menu-item>
      <a-menu-item>
        <a-icon type="setting" />
        <span>设置</span>
      </a-menu-item> -->
      <!-- <a-menu-divider /> -->
      <a-menu-item @click="logout">
        <a-icon style="margin-right: 8px;" type="poweroff" />
        <span>退出登录</span>
      </a-menu-item>
    </a-menu>
  </a-dropdown>
</template>

<script>
import {mapGetters} from 'vuex'
import {logout} from '@/services/user'

export default {
  name: 'HeaderAvatar',
  computed: {
    ...mapGetters('account', ['user']),
    // 获取头像显示文字（用户名首字母或首个汉字）
    avatarText() {
      if (!this.user || !this.user.name) return 'U'
      const name = this.user.name
      // 如果是中文，取第一个字
      if (/[\u4e00-\u9fa5]/.test(name)) {
        return name.charAt(0)
      }
      // 如果是英文，取首字母大写
      return name.charAt(0).toUpperCase()
    },
    // 根据用户名生成固定的头像背景色
    avatarColor() {
      const colors = [
        '#f56a00', '#7265e6', '#ffbf00', '#00a2ae',
        '#1890ff', '#52c41a', '#eb2f96', '#722ed1',
        '#13c2c2', '#fa541c', '#2f54eb', '#faad14'
      ]
      if (!this.user || !this.user.name) return colors[0]
      // 根据用户名生成一个固定的颜色索引
      let hash = 0
      for (let i = 0; i < this.user.name.length; i++) {
        hash = this.user.name.charCodeAt(i) + ((hash << 5) - hash)
      }
      return colors[Math.abs(hash) % colors.length]
    }
  },
  methods: {
    logout() {
      logout()
      this.$router.push('/login')
    }
  }
}
</script>

<style lang="less">
  .header-avatar{
    display: inline-flex;
    .avatar, .name{
      align-self: center;
    }
    .avatar{
      margin-right: 8px;
      font-weight: 600;
      font-size: 12px;
    }
    .name{
      font-weight: 500;
    }
  }
  .avatar-menu{
    width: 150px;
  }

</style>
