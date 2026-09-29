<template>
  <div class="promotion-container">
    <div class="content" v-loading="pageLoading">
      <template v-if="loaded">
        <!-- ===== 我的推广：推广码 + 推广链接 + 收益概览 ===== -->
        <div class="card">
          <h3 class="section-title">
            <el-icon><Promotion /></el-icon>
            我的推广
          </h3>

          <div class="link-row">
            <span class="row-label">推广码</span>
            <el-input :model-value="affCode" readonly class="link-input" placeholder="推广码生成中…" />
            <el-button type="primary" plain :disabled="!affCode" @click="copyCode">复制推广码</el-button>
          </div>

          <div class="link-row">
            <span class="row-label">推广链接</span>
            <el-input :model-value="promoLink" readonly class="link-input" placeholder="推广链接生成中…" />
            <el-button type="primary" :disabled="!promoLink" @click="copyLink">复制链接</el-button>
          </div>

          <div class="stat-row">
            <div class="stat">
              <span class="stat-label">累计提成</span>
              <span class="stat-value">¥{{ money(finance.total_commission) }}</span>
            </div>
            <div class="stat">
              <span class="stat-label">已提现</span>
              <span class="stat-value">¥{{ money(finance.withdrawn) }}</span>
            </div>
            <div class="stat">
              <span class="stat-label">可提现</span>
              <span class="stat-value primary">¥{{ money(finance.available) }}</span>
            </div>
            <div class="stat">
              <span class="stat-label">待审核提现</span>
              <span class="stat-value">{{ pendingWithdraw }} 笔</span>
            </div>
            <div class="stat">
              <span class="stat-label">推广客户</span>
              <span class="stat-value">{{ subUserCount }}</span>
            </div>
          </div>

          <p class="promo-tip">
            通过您的专属推广链接注册的用户将计入您的推广业绩，其每一笔消费与购包都将按比例为您产生提成收益；
            提成比例由平台统一配置，调整后立即生效。
          </p>
        </div>

        <!-- ===== 提成流水 / 推广客户 / 提现记录 ===== -->
        <div class="card">
          <div class="card-head">
            <el-radio-group v-model="tab" @change="onTabChange">
              <el-radio-button value="income">提成流水</el-radio-button>
              <el-radio-button value="users">推广客户</el-radio-button>
              <el-radio-button value="withdraws">提现记录</el-radio-button>
            </el-radio-group>
            <el-button v-if="tab === 'income'" type="primary" plain @click="openWithdraw">申请提现</el-button>
          </div>

          <!-- 提成流水 -->
          <el-table v-if="tab === 'income'" :data="incomeList" v-loading="listLoading" style="width: 100%">
            <el-table-column label="时间" min-width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="类型" width="130">
              <template #default="{ row }">{{ bizLabel(row.biz_type) }}</template>
            </el-table-column>
            <el-table-column label="关联用户" width="110">
              <template #default="{ row }">{{ row.user_id || '-' }}</template>
            </el-table-column>
            <el-table-column label="收益金额" width="130">
              <template #default="{ row }">
                <span :class="{ minus: Number(row.amount) < 0 }">{{ signed(row.amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="remark" label="说明" min-width="200" />
            <template #empty>
              <span class="empty-text">暂无收益记录</span>
            </template>
          </el-table>

          <!-- 推广客户 -->
          <el-table v-else-if="tab === 'users'" :data="userList" v-loading="listLoading" style="width: 100%">
            <el-table-column prop="id" label="用户 ID" width="100" />
            <el-table-column prop="username" label="用户名" min-width="140" show-overflow-tooltip />
            <el-table-column prop="phone" label="手机号" width="140" />
            <el-table-column label="实名状态" width="110">
              <template #default="{ row }">
                <el-tag size="small" :type="row.realname_status >= 1 ? 'success' : 'warning'">
                  {{ realnameText(row.realname_status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="注册时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <template #empty>
              <span class="empty-text">暂无推广客户</span>
            </template>
          </el-table>

          <!-- 提现记录 -->
          <el-table v-else :data="withdrawList" v-loading="listLoading" style="width: 100%">
            <el-table-column label="时间" min-width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="提现方式" width="110">
              <template #default="{ row }">{{ channelLabel(row.channel) }}</template>
            </el-table-column>
            <el-table-column label="提现金额" width="120">
              <template #default="{ row }">¥{{ money(row.amount) }}</template>
            </el-table-column>
            <el-table-column label="手续费" width="110">
              <template #default="{ row }">¥{{ money(row.fee) }}</template>
            </el-table-column>
            <el-table-column label="实际到账" width="120">
              <template #default="{ row }">¥{{ money(row.actual_amount) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag size="small" :type="withdrawStatusTag(row.status)">
                  {{ withdrawStatusText(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="到账时间" min-width="170">
              <template #default="{ row }">{{ row.paid_at ? formatDateTime(row.paid_at) : '-' }}</template>
            </el-table-column>
            <template #empty>
              <span class="empty-text">暂无提现记录</span>
            </template>
          </el-table>

          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @current-change="loadList"
            @size-change="handleSizeChange"
            class="pager"
          />
        </div>
      </template>
    </div>

    <!-- 提现申请弹窗 -->
    <el-dialog v-model="withdrawVisible" title="申请提现" width="480px">
      <el-form label-width="90px">
        <el-form-item label="可提现">
          <span class="withdraw-available">¥{{ money(finance.available) }}</span>
        </el-form-item>
        <el-form-item label="提现方式">
          <el-radio-group v-model="withdrawForm.channel">
            <el-radio value="balance">余额</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="提现金额">
          <el-input-number v-model="withdrawForm.amount" :min="0" :precision="2" :step="100" style="width: 100%" />
        </el-form-item>
      </el-form>
      <p class="withdraw-tip">提现到余额为站内划转：即时到账、免手续费，到账后可直接用于消费或在线支付。</p>
      <template #footer>
        <el-button @click="withdrawVisible = false">取消</el-button>
        <el-button type="primary" :loading="withdrawSubmitting" @click="submitWithdraw">确认提现</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Promotion } from '@element-plus/icons-vue'
import { promotionAPI } from '@/api'
import { formatDateTime } from '@/utils/format'
import { siteBase } from '@/utils/promotion'

type TabKey = 'income' | 'users' | 'withdraws'

// 提成业务类型文案
const BIZ_LABELS: Record<string, string> = {
  pack_purchase: '资源包购买',
  sms_send: '短信发送',
  fv_auth: '人脸核验',
  refund: '退款冲回'
}

const pageLoading = ref(true)
const loaded = ref(false)

const profile = ref<any>({})
const affCode = computed(() => String(profile.value.aff_code || ''))
const finance = computed(() => profile.value.finance || {})
const subUserCount = computed(() => Number(profile.value.sub_user_count || 0))
const pendingWithdraw = computed(() => Number(finance.value.pending_withdraw || 0))

// 推广链接：控制台注册地址 + 推广码（?ref=XXXX），码未生成时不给可用链接
const promoLink = computed(() => (affCode.value ? `${siteBase('console')}/register?ref=${affCode.value}` : ''))

// ===== 列表（提成流水 / 推广客户 / 提现记录共用分页） =====
const tab = ref<TabKey>('income')
const listLoading = ref(false)
const incomeList = ref<any[]>([])
const userList = ref<any[]>([])
const withdrawList = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const money = (v: any) => Number(v || 0).toFixed(2)

const signed = (v: any) => {
  const n = Number(v || 0)
  if (n === 0) return '-'
  return (n > 0 ? '+' : '') + n.toFixed(2)
}

const bizLabel = (biz: string) => BIZ_LABELS[biz] || biz || '-'
const realnameText = (status: number) => (status === 2 ? '企业实名' : status === 1 ? '个人实名' : '未实名')

// 提现方式与提现状态文案
const CHANNEL_LABELS: Record<string, string> = { balance: '余额' }
const channelLabel = (channel: string) => CHANNEL_LABELS[channel] || channel || '-'

const WITHDRAW_STATUS: Record<number, { text: string; tag: 'warning' | 'primary' | 'success' | 'danger' | 'info' }> = {
  0: { text: '待审核', tag: 'warning' },
  1: { text: '待打款', tag: 'primary' },
  2: { text: '已完成', tag: 'success' },
  3: { text: '已驳回', tag: 'danger' }
}
const withdrawStatusText = (status: number) => WITHDRAW_STATUS[status]?.text || '-'
const withdrawStatusTag = (status: number) => WITHDRAW_STATUS[status]?.tag || 'info'

const loadList = async () => {
  listLoading.value = true
  try {
    if (tab.value === 'income') {
      const res: any = await promotionAPI.financeLogs({ page: page.value, page_size: pageSize.value })
      incomeList.value = res?.list || []
      total.value = Number(res?.total || 0)
    } else if (tab.value === 'users') {
      const res: any = await promotionAPI.subUsers({ page: page.value, page_size: pageSize.value })
      userList.value = res?.list || []
      total.value = Number(res?.total || 0)
    } else {
      const res: any = await promotionAPI.withdraws({ page: page.value, page_size: pageSize.value })
      withdrawList.value = res?.list || []
      total.value = Number(res?.total || 0)
    }
  } catch (error) {
    console.error(error)
  } finally {
    listLoading.value = false
  }
}

const onTabChange = () => {
  page.value = 1
  loadList()
}

const handleSizeChange = () => {
  page.value = 1
  loadList()
}

const load = async () => {
  try {
    const res: any = await promotionAPI.profile()
    profile.value = res || {}
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
    loaded.value = true
    loadList()
  }
}

const copyCode = async () => {
  if (!affCode.value) return
  try {
    await navigator.clipboard.writeText(affCode.value)
    ElMessage.success('推广码已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

const copyLink = async () => {
  if (!promoLink.value) return
  try {
    await navigator.clipboard.writeText(promoLink.value)
    ElMessage.success('推广链接已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择链接复制')
  }
}

// ===== 提现 =====
const withdrawVisible = ref(false)
const withdrawSubmitting = ref(false)
const withdrawForm = reactive({ amount: 0, channel: 'balance' })

const openWithdraw = () => {
  withdrawForm.amount = Number(finance.value.available || 0)
  withdrawForm.channel = 'balance'
  withdrawVisible.value = true
}

const submitWithdraw = async () => {
  if (withdrawForm.amount <= 0) {
    ElMessage.warning('请输入提现金额')
    return
  }
  if (withdrawForm.amount > Number(finance.value.available || 0)) {
    ElMessage.warning('提现金额超过可提现收益')
    return
  }
  withdrawSubmitting.value = true
  try {
    await promotionAPI.withdraw({ amount: withdrawForm.amount, channel: withdrawForm.channel })
    ElMessage.success('提现成功，收益已转入平台余额')
    withdrawVisible.value = false
    await load()
  } catch (error: any) {
    ElMessage.error(error?.message || '提现失败')
  } finally {
    withdrawSubmitting.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.promotion-container {
  min-height: 100%;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.promo-tip {
  margin: 20px 0 0;
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.8;
}

.link-row {
  display: flex;
  gap: 12px;
  align-items: center;
}

.link-row + .link-row {
  margin-top: 12px;
}

.row-label {
  flex: none;
  width: 68px;
  font-size: 13px;
  color: var(--text-muted);
}

.link-input {
  flex: 1;
  max-width: 640px;
}

.stat-row {
  display: flex;
  gap: 48px;
  margin-top: 22px;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
}

.stat-value {
  font-size: 22px;
  font-weight: 600;
}

.stat-value.primary {
  color: #409eff;
}

.pager {
  margin-top: 20px;
  justify-content: center;
}

.empty-text {
  color: var(--text-muted);
  font-size: 13px;
}

.minus {
  color: #f56c6c;
}

.withdraw-available {
  font-size: 16px;
  font-weight: 600;
  color: #409eff;
}

.withdraw-tip {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}
</style>