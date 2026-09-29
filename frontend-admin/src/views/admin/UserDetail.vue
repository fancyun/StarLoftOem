<template>
  <div class="admin-user-detail">
    <div class="card">
      <div class="card-head" style="display: flex; align-items: center; gap: 12px;">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回用户管理
        </el-button>
        <h3 class="section-title" style="margin: 0;">用户详情</h3>
      </div>

      <div v-loading="loading" style="min-height: 200px">
        <!-- 用户信息 -->
        <el-descriptions :column="2" border v-if="user.id">
          <el-descriptions-item label="用户ID">{{ user.id }}</el-descriptions-item>
          <el-descriptions-item label="用户名">{{ user.username || '-' }}</el-descriptions-item>
          <el-descriptions-item label="手机号">{{ user.phone }}</el-descriptions-item>
          <el-descriptions-item label="账户余额">¥{{ user.balance }}</el-descriptions-item>
          <el-descriptions-item label="账号状态">
            <el-tag :type="user.status === 1 ? 'success' : 'danger'">
              {{ user.status === 1 ? '正常' : '禁用' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="注册时间">{{ formatDateTime(user.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="最后登录">{{ user.last_login_at ? formatDateTime(user.last_login_at) : '-' }}</el-descriptions-item>
        </el-descriptions>

        <!-- 实名认证 -->
        <h4 style="margin-top: 24px;">实名认证</h4>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="个人实名">
            <div style="display: flex; align-items: center; gap: 12px;">
              <template v-if="user.realname_status === 1">
                <el-tag type="success">已实名</el-tag>
                <span v-if="user.verified_name">{{ maskName(user.verified_name) }}</span>
                <span v-if="user.verified_number">{{ maskIdCard(user.verified_number) }}</span>
              </template>
              <el-tag v-else type="info">未实名</el-tag>
              <span style="margin-left: auto;">剩余免费次数：<b>{{ user.personal_free_remaining >= 0 ? user.personal_free_remaining : '-' }} 次</b></span>
              <el-button v-if="canWrite" size="small" type="primary" plain @click="handleResetKycFree('personal')">重置</el-button>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="企业实名">
            <div style="display: flex; align-items: center; gap: 12px;">
              <el-tag :type="user.realname_status === 2 ? 'success' : 'info'">
                {{ user.realname_status === 2 ? '已实名' : '未实名' }}
              </el-tag>
              <span style="margin-left: auto;">剩余免费次数：<b>{{ user.enterprise_free_remaining >= 0 ? user.enterprise_free_remaining : '-' }} 次</b></span>
              <el-button v-if="canWrite" size="small" type="primary" plain @click="handleResetKycFree('enterprise')">重置</el-button>
            </div>
          </el-descriptions-item>
        </el-descriptions>

        <!-- 操作按钮 -->
        <div v-if="canWrite" style="margin-top: 20px; display: flex; gap: 12px;">
          <el-button type="primary" @click="handleRecharge">充值</el-button>
          <el-button :type="user.status === 1 ? 'warning' : 'success'" @click="handleToggleStatus">
            {{ user.status === 1 ? '禁用' : '启用' }}
          </el-button>
        </div>

        <!-- 财务概览 -->
        <h4 style="margin-top: 28px;">财务概览</h4>
        <el-descriptions :column="4" border>
          <el-descriptions-item label="当前余额">¥{{ financeStats.balance ?? user.balance }}</el-descriptions-item>
          <el-descriptions-item label="总充值金额">¥{{ financeStats.totalRecharge }}</el-descriptions-item>
          <el-descriptions-item label="总消费金额">¥{{ financeStats.totalConsume }}</el-descriptions-item>
          <el-descriptions-item label="总退款金额">¥{{ financeStats.totalRefund }}</el-descriptions-item>
        </el-descriptions>

        <!-- 流水与订单 -->
        <el-tabs v-model="activeTab" style="margin-top: 16px;">
          <el-tab-pane label="余额流水" name="balance">
            <el-table :data="balanceLogs" style="width: 100%" v-loading="tableLoading">
              <el-table-column prop="created_at" label="时间" width="180">
                <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
              </el-table-column>
              <el-table-column prop="type" label="类型" width="100">
                <template #default="{ row }">
                  <el-tag :type="getBalanceTypeTag(row.type)">{{ getBalanceTypeName(row.type) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="amount" label="金额" width="120">
                <template #default="{ row }">
                  <span :style="{ color: isIncome(row.type) ? 'var(--color-success)' : 'var(--color-danger)' }">
                    {{ isIncome(row.type) ? '+' : '-' }}¥{{ row.amount }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="balance_after" label="余额" width="120" />
              <el-table-column prop="bank_serial_no" label="银行流水单号" width="180">
                <template #default="{ row }">{{ row.bank_serial_no || '-' }}</template>
              </el-table-column>
              <el-table-column prop="remark" label="备注" />
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="认证记录" name="orders">
            <el-table :data="authOrders" style="width: 100%" v-loading="tableLoading">
              <el-table-column prop="biz_no" label="业务流水号" width="200" />
              <el-table-column label="资源包扣减" width="100">
                <template #default="{ row }">{{ row.pack_count ? row.pack_count + ' 次' : '-' }}</template>
              </el-table-column>
              <el-table-column label="金额" width="100">
                <template #default="{ row }">{{ row.pay_type === 2 ? '资源包' : (row.cost ? '¥' + row.cost : '-') }}</template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="getOrderStatusTag(row.status)">{{ getOrderStatusName(row.status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="created_at" label="创建时间" width="180">
                <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <!-- 人工充值对话框 -->
    <el-dialog v-model="rechargeDialogVisible" title="人工充值" width="500px">
      <el-form :model="rechargeForm" :rules="rechargeRules" ref="rechargeFormRef" label-width="100px">
        <el-form-item label="用户手机号">
          <el-input :value="rechargeForm.phone" disabled />
        </el-form-item>
        <el-form-item label="当前余额">
          <el-input :value="`¥${rechargeForm.currentBalance}`" disabled />
        </el-form-item>
        <el-form-item label="充值金额" prop="amount">
          <el-input-number
            v-model="rechargeForm.amount"
            :precision="2"
            :step="10"
            :min="0.01"
            :max="10000"
            placeholder="请输入充值金额"
            style="width: 100%"
          />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            单位：元，最小0.01元，最大10000元
          </div>
        </el-form-item>
        <el-form-item label="记账日期" prop="book_date">
          <el-input
            v-model="rechargeForm.book_date"
            placeholder="粘贴银行对账单记账日期，如 2026-08-21-12.44.19.996191"
            maxlength="32"
            @blur="formatBookDate"
          />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            自动转为纯数字（如 2026-08-21-12.44.19.996191 → 20260821124419996191）
          </div>
        </el-form-item>
        <el-form-item label="账号" prop="bank_account">
          <el-input
            v-model="rechargeForm.bank_account"
            placeholder="请输入银行账号（卡号，必填）"
            maxlength="50"
          />
        </el-form-item>
        <el-form-item label="交易流水号" prop="trade_serial">
          <el-input
            v-model="rechargeForm.trade_serial"
            placeholder="请输入交易流水号（必填）"
            maxlength="50"
          />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            提交后按「账号_记账时间_交易流水号」拼接，唯一账单即按此组合校验
          </div>
        </el-form-item>
        <el-form-item label="备注" prop="remark">
          <el-input
            v-model="rechargeForm.remark"
            type="textarea"
            :rows="3"
            placeholder="请输入充值备注（可选）"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRechargeSubmit" :loading="rechargeLoading">确定充值</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const adminStore = useAdminStore()
// 写权限：无 sys.users.write 时隐藏充值/发放/启停/重置免费次数按钮
const canWrite = computed(() => adminStore.has('sys.users.write'))

const loading = ref(false)
const tableLoading = ref(false)
const user = reactive<any>({})
const activeTab = ref('balance')

const financeStats = reactive({
  balance: 0,
  totalRecharge: 0,
  totalConsume: 0,
  totalRefund: 0
})
const balanceLogs = ref([])
const authOrders = ref([])

const goBack = () => {
  router.push('/sys/users')
}

const loadUser = async () => {
  loading.value = true
  try {
    const detail: any = await adminAPI.getUserDetail(Number(route.params.id))
    Object.assign(user, detail)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '加载用户详情失败')
  } finally {
    loading.value = false
  }
}

const loadFinance = async () => {
  tableLoading.value = true
  try {
    const stats: any = await adminAPI.getUserFinanceStats(user.id)
    Object.assign(financeStats, stats)
  } catch (error) {
    console.error('Failed to load finance stats:', error)
  }
  try {
    const logs: any = await adminAPI.getUserBalanceLogs(user.id)
    balanceLogs.value = logs.list || logs || []
  } catch (error) {
    console.error('Failed to load balance logs:', error)
    balanceLogs.value = []
  }
  try {
    const orders: any = await adminAPI.getUserAuthRecords(user.id)
    authOrders.value = orders.list || orders || []
  } catch (error) {
    console.error('Failed to load auth orders:', error)
    authOrders.value = []
  }
  tableLoading.value = false
}

const handleToggleStatus = async () => {
  const action = user.status === 1 ? '禁用' : '启用'
  try {
    await ElMessageBox.confirm(`确认${action}该用户？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await adminAPI.updateUserStatus(user.id, { status: user.status === 1 ? 0 : 1 })
    ElMessage.success(`${action}成功`)
    user.status = user.status === 1 ? 0 : 1
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(`${action}失败`)
    }
  }
}

const maskName = (name: string) => {
  if (!name || name.length === 0) return ''
  if (name.length === 1) return name
  if (name.length === 2) return name[0] + '*'
  return name[0] + '*'.repeat(name.length - 2) + name[name.length - 1]
}

const maskIdCard = (idCard: string) => {
  if (!idCard || idCard.length < 8) return idCard
  return idCard.substring(0, 3) + '***********' + idCard.substring(idCard.length - 4)
}

// 充值
const rechargeDialogVisible = ref(false)
const rechargeLoading = ref(false)
const rechargeFormRef = ref<FormInstance>()

const rechargeForm = reactive({
  userId: 0,
  phone: '',
  currentBalance: 0,
  amount: undefined as number | undefined,
  book_date: '',
  bank_account: '',
  trade_serial: '',
  remark: ''
})

// 记账日期归一化：支持从银行对账单直接粘贴，提取「日期 + 时分秒（可带小数秒）」并统一转为纯数字。
// 仅日期 → 8 位 yyyyMMdd；带时分 → 14 位 yyyyMMddHHmmss（缺秒补 00）；带毫秒/微秒 → 追加对应位数；无法识别返回空串。
const bookDatePattern = /(\d{4})\D(\d{1,2})\D(\d{1,2})(?:\D+(\d{1,2})\D(\d{1,2})(?:\D+(\d{1,2}))?(?:\D+(\d{1,6}))?)?/

const normalizeBookDate = (v: string): string => {
  const s = String(v || '').trim()
  if (!s) return ''
  // 失焦回写后的纯数字：按长度校验并补足秒
  if (/^\d+$/.test(s)) {
    if (s.length === 8) return s
    if (s.length === 12) return s + '00'
    if (s.length === 14 || s.length === 17 || s.length === 20) return s
    return ''
  }
  const m = s.match(bookDatePattern)
  if (!m) return ''
  const pad = (part: string, len = 2) => part.padStart(len, '0')
  let out = m[1] + pad(m[2]) + pad(m[3])
  if (m[4] && m[5]) {
    out += pad(m[4]) + pad(m[5]) + (m[6] ? pad(m[6]) : '00')
  }
  // 小数秒按十进制补齐到 6 位（毫秒 .996 = 996000 微秒），避免不同精度写法撞成同一个键
  if (m[7]) out += m[7].padEnd(6, '0')
  return out
}

// 记账日期失焦时回写归一化结果，便于管理员确认转换后的时间戳
const formatBookDate = () => {
  const normalized = normalizeBookDate(rechargeForm.book_date)
  if (normalized) rechargeForm.book_date = normalized
}

const rechargeRules: FormRules = {
  amount: [
    { required: true, message: '请输入充值金额', trigger: 'blur' },
    { type: 'number', min: 0.01, max: 10000, message: '充值金额范围为0.01-10000元', trigger: 'blur' }
  ],
  book_date: [
    { required: true, message: '请输入记账日期', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, cb: any) =>
        !value || normalizeBookDate(value) ? cb() : cb(new Error('记账日期格式有误，请粘贴如 2026-08-21-12.44.19.996191')),
      trigger: 'blur'
    }
  ],
  bank_account: [
    { required: true, message: '请输入银行账号', trigger: 'blur' },
    { min: 4, message: '银行账号至少4位', trigger: 'blur' }
  ],
  trade_serial: [
    { required: true, message: '请输入交易流水号', trigger: 'blur' },
    { min: 2, message: '交易流水号至少2个字符', trigger: 'blur' }
  ]
}

const handleRecharge = () => {
  rechargeForm.userId = user.id
  rechargeForm.phone = user.phone
  rechargeForm.currentBalance = user.balance
  rechargeForm.amount = undefined
  rechargeForm.book_date = ''
  rechargeForm.bank_account = ''
  rechargeForm.trade_serial = ''
  rechargeForm.remark = ''
  rechargeDialogVisible.value = true
  rechargeFormRef.value?.clearValidate()
}

const handleRechargeSubmit = async () => {
  if (!rechargeFormRef.value) return
  await rechargeFormRef.value.validate(async (valid) => {
    if (!valid) return
    rechargeLoading.value = true
    try {
      // 唯一账单 = 账号 + 记账时间（纯数字）+ 银行交易流水号；三段各自去掉前后空格后拼接，再由后端做全局唯一校验
      const bankSerialNo = [
        String(rechargeForm.bank_account || '').trim(),
        normalizeBookDate(rechargeForm.book_date),
        String(rechargeForm.trade_serial || '').trim()
      ].join('_')
      const data = {
        amount: rechargeForm.amount!,
        bank_serial_no: bankSerialNo,
        remark: rechargeForm.remark || undefined
      }
      await adminAPI.rechargeUser(rechargeForm.userId, data)
      ElMessage.success('充值成功')
      rechargeDialogVisible.value = false
      await loadUser()
      await loadFinance()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || '充值失败')
    } finally {
      rechargeLoading.value = false
    }
  })
}

// 重置实名次数
const handleResetKycFree = async (type: 'personal' | 'enterprise') => {
  const typeName = type === 'personal' ? '个人实名' : '企业实名'
  try {
    await ElMessageBox.confirm(`确认重置该用户的${typeName}免费次数？重置后剩余免费次数恢复为 3 次。`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await adminAPI.resetUserKycFree(user.id, type)
    ElMessage.success(`${typeName}免费次数已重置`)
    await loadUser()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '重置失败')
    }
  }
}

// 财务展示辅助
const getBalanceTypeName = (type: number | string) => {
  const names: Record<number, string> = { 1: '充值', 2: '消费', 3: '退款' }
  return names[Number(type)] || String(type)
}

const getBalanceTypeTag = (type: number | string) => {
  const tags: Record<number, string> = { 1: 'success', 2: 'danger', 3: 'warning' }
  return tags[Number(type)] || 'info'
}

const isIncome = (type: number | string) => [1, 3].includes(Number(type))

const getOrderStatusName = (status: number) => {
  const names: Record<number, string> = {
    0: '待认证',
    1: '认证中',
    2: '已完成',
    3: '失败',
    4: '已取消',
    5: '超时已退款',
    6: '发起失败（未扣费）'
  }
  return names[status] || String(status)
}

const getOrderStatusTag = (status: number) => {
  const tags: Record<number, string> = {
    0: 'info',
    1: 'warning',
    2: 'success',
    3: 'danger',
    4: 'info',
    5: 'info',
    6: 'info'
  }
  return tags[status] || 'info'
}

onMounted(() => {
  loadUser().then(() => loadFinance())
})
</script>
