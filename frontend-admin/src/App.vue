<template>
  <div id="app-root">
    <router-view />
  </div>
</template>

<script setup lang="ts">
// App 保持最轻：不做 html/body 背景覆盖，由 main.css + base.css 变量驱动
// 这样 Element Plus 原生浅主题和页面背景一致，不会出现割裂
</script>

<style scoped>
#app-root {
  min-height: 100vh;
  /* 不强行覆盖 body，避免 base.css 定义的浅色变量被暗色强值覆盖造成不协调 */
}
</style>
