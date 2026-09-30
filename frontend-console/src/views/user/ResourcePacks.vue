<template>
  <div class="packs-container">
    <div class="content" v-loading="pageLoading">
      <!-- 资源包概览 + 购买入口 -->
      <div class="card">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Wallet /></el-icon>
            我的资源
          </h3>
          <el-button type="primary" @click="$router.push($route.path + '/buy')">
            <el-icon><ShoppingCart /></el-icon>
            <span class="buy-btn-text">购买资源包</span>
          </el-button>
        </div>
        <div class="balance-display">
          <span class="balance-label">当前余额</span>
          <span class="balance-amount">¥{{ userInfo.balance }}</span>
          <span class="balance-tip">购买资源包将直接从余额扣费，请先充值再购买</span>
        </div>
      </div>

      <!-- 我的资源包 -->
      <div class="card">
        <h3 class="section-title">
          <el-icon><Box /></el-icon>
          我的资源包
        </h3>
        <div v-if="myPacks.length" class="my-pack-list">
          <div
            v-for="pack in myPacks"
            :key="pack.id"
            class="my-pack-item"
            :class="{ empty: pack.remaining_count <= 0 }"
          >
            <div class="my-pack-info">
              <div class="my-pack-name">
                {{ productLabel(pack.product) }}
              </div>
              <div class="my-pack-count">
                剩余 {{ pack.remaining_count }} / {{ pack.total_count }} {{ unitText(pack.product) }}
              </div>
            </div>
            <el-tag :type="pack.remaining_count > 0 ? 'success' : 'info'">
              {{ pack.remaining_count > 0 ? '可用' : '已用完' }}
            </el-tag>
          </div>
        </div>
        <el-empty v-else description="暂无资源包，点击右上角「购买资源包」前往购买" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Wallet, Box, ShoppingCart } from '@element-plus/icons-vue'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const userInfo = ref<any>({})
const myPacks = ref<any[]>([])
const pageLoading = ref(true)

const productLabel = (product: string) => {
  if (product === 'sms') return '短信服务'
  if (product === 'fv_auth') return '人脸核验-有源'
  if (product === 'fv_self') return '人脸核验-无源'
  return product || ''
}

const unitText = (product: string) => (product === 'sms' ? '条短信' : '次')

const loadData = async () => {
  try {
    const [profile, mine] = await Promise.all([
      userAPI.getProfile(),
      userAPI.myPacks()
    ])
    userInfo.value = profile
    userStore.setUserInfo(profile)
    myPacks.value = mine.list || []
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.packs-container {
  min-height: 100%;
}

.content {
  max-width: 900px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.buy-btn-text {
  margin-left: 4px;
}

.balance-display {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px;
  background: linear-gradient(135deg, var(--color-primary-light) 0%, #D6E8FF 100%);
  border-radius: var(--radius-md);
  margin-top: 20px;
}

.balance-label {
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 8px;
}

.balance-amount {
  color: var(--text-primary);
  font-size: 32px;
  font-weight: 700;
}

.balance-tip {
  margin-top: 8px;
  color: var(--text-muted);
  font-size: 12px;
}

/* 我的资源包 */
.my-pack-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.my-pack-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  background: var(--bg-page);
}

.my-pack-item.empty {
  opacity: 0.6;
}

.my-pack-name {
  font-weight: 600;
  color: var(--text-primary);
}

.my-pack-count {
  font-size: 13px;
  color: var(--text-secondary);
  margin-top: 4px;
}
</style>
