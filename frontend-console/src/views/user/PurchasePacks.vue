<template>
  <div class="packs-container">
    <div class="content" v-loading="pageLoading">
      <div class="card">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><ShoppingCart /></el-icon>
            购买资源包
          </h3>
          <el-button @click="$router.back()">返回</el-button>
        </div>
        <p class="purchase-tip">支持余额支付与支付宝/微信组合支付（余额不足时自动补差额），支付完成后次数实时到账。</p>

        <div class="product-tabs">
          <el-radio-group v-model="activeProduct" @change="onTabChange">
            <el-radio-button value="fv_auth">有源（公安库）</el-radio-button>
            <el-radio-button value="fv_self">无源（自传照片）</el-radio-button>
          </el-radio-group>
          <el-radio-group v-model="payChannel" class="pay-channel">
            <el-radio-button value="balance">余额</el-radio-button>
            <el-radio-button v-for="ch in payChannelOptions" :key="ch.value" :value="ch.value">{{ ch.label }}</el-radio-button>
          </el-radio-group>
        </div>

        <div v-if="packs.length" class="pack-grid">
          <div v-for="pack in packs" :key="pack.id" class="pack-card">
            <div class="pack-header">
              <div class="pack-name">{{ pack.name }}</div>
              <el-tag size="small" type="primary">{{ productLabel(pack.product) }}</el-tag>
            </div>
            <div class="pack-count">{{ pack.total_count }} {{ unitText() }}</div>
            <div class="pack-desc">{{ pack.description || defaultDesc() }}</div>
            <div class="pack-footer">
              <span class="pack-price">¥{{ packPrice(pack) }}</span>
              <el-button
                type="primary"
                size="small"
                :loading="purchasingId === pack.id"
                @click="handlePurchase(pack)"
              >
                购买
              </el-button>
            </div>
          </div>
        </div>
        <el-empty v-else description="暂无可购买的资源包" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ShoppingCart } from '@element-plus/icons-vue'
import { userAPI } from '@/api'
import { PAY_CHANNEL_LABELS, PAY_CHANNEL_OPTIONS, loadEnabledPayChannels } from '@/utils/payment'

const router = useRouter()
const packs = ref<any[]>([])
// 资源包 ID -> 当前用户适用售价（推广/单用户定价优先）
const packPrices = ref<Record<string, number>>({})
const purchasingId = ref<number | null>(null)
const pageLoading = ref(true)
const activeProduct = ref('fv_auth')
const payChannel = ref('balance')

const packPrice = (pack: any) => packPrices.value[pack.id] ?? pack.price

// 已启用的在线支付渠道（未配置凭据的渠道不展示）
const enabledChannels = ref<string[]>(['alipay', 'wechat'])
const payChannelOptions = computed(() => PAY_CHANNEL_OPTIONS.filter((ch) => enabledChannels.value.includes(ch.value)))

const productLabel = (product: string) => {
  if (product === 'fv_auth') return '人脸核验-有源'
  if (product === 'fv_self') return '人脸核验-无源'
  return product || ''
}

const unitText = () => '次认证'

const defaultDesc = () => '人脸核验认证次数'

// 移动端微信走 H5 跳转，PC 端走 Native 扫码
const isMobile = () => {
  return (
    /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
    (navigator.maxTouchPoints > 1 && window.innerWidth < 768)
  )
}

const loadPacks = async () => {
  pageLoading.value = true
  try {
    const onSale = await userAPI.listPacks(activeProduct.value)
    packs.value = onSale.list || []
    packPrices.value = onSale.prices || {}
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
  }
}

const onTabChange = () => {
  loadPacks()
}

const handlePurchase = (pack: any) => {
  const methodText = payChannel.value === 'balance' ? '余额' : PAY_CHANNEL_LABELS[payChannel.value] || payChannel.value
  ElMessageBox.confirm(
    `确认使用${methodText} ¥${packPrice(pack)} 购买「${pack.name}」？${payChannel.value === 'balance' ? '' : '余额不足时将自动补差额完成组合支付，'}购买后将获得 ${pack.total_count} ${unitText()}。`,
    '确认购买',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
    .then(async () => {
      purchasingId.value = pack.id
      try {
        if (payChannel.value === 'balance') {
          await userAPI.purchasePack(pack.id)
          ElMessage.success('购买成功')
          await loadPacks()
          return
        }
        // 在线购买（余额 + 在线支付渠道组合支付）
        const scene = payChannel.value === 'wechat' && !isMobile() ? 'native' : 'h5'
        const res: any = await userAPI.payPack(pack.id, { channel: payChannel.value, scene })
        if (res.fully_paid) {
          // 余额已全额覆盖，直接发放资源包
          ElMessage.success('购买成功')
          await loadPacks()
          return
        }
        // 需要在线支付：携带支付信息跳转支付详情页
        sessionStorage.setItem(
          `starloft_pay_${res.pay_order_no}`,
          JSON.stringify({ ...res, pay_purpose: 'resource_pack', return_path: '/fv/packs' })
        )
        router.push(`/payment/${res.pay_order_no}`)
      } catch (error: any) {
        ElMessage.error(error?.message || '购买失败')
      } finally {
        purchasingId.value = null
      }
    })
    .catch(() => {})
}

onMounted(async () => {
  loadPacks()
  enabledChannels.value = await loadEnabledPayChannels()
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

.purchase-tip {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 20px;
  padding: 10px 14px;
  background: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.product-tabs {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

/* 在售资源包：一行 3 个 */
.pack-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.pack-card {
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  padding: 20px;
  background: var(--bg-page);
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: box-shadow 0.15s;
}

.pack-card:hover {
  box-shadow: var(--shadow-md);
}

.pack-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.pack-name {
  font-weight: 700;
  color: var(--text-primary);
  font-size: 16px;
}

.pack-count {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-primary);
}

.pack-desc {
  font-size: 13px;
  color: var(--text-secondary);
  min-height: 36px;
}

.pack-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 8px;
  border-top: 1px solid var(--border-light);
}

.pack-price {
  font-size: 18px;
  font-weight: 700;
  color: var(--color-danger);
}
</style>
