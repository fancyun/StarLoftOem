<template>
  <div class="sms-send-container page-fill">
    <div class="content">
      <div class="card">
        <h3 class="section-title">
          <el-icon><Promotion /></el-icon>
          在线发送
        </h3>
        <p class="send-tip">
          使用本账号「已审核通过」的签名与模板在线发送短信（仅支持模板发送）。发送成功后按实际计费条数扣减短信资源包条数，
          不足则扣余额；明细与回执可在「发送记录」查看。
        </p>

        <el-form :model="form" label-width="90px" class="send-form" v-loading="loading">
          <el-form-item label="短信签名">
            <el-select v-model="form.sign_name" placeholder="留空则使用模板绑定的签名" clearable style="max-width: 360px">
              <el-option
                v-for="sign in signs"
                :key="sign.id"
                :label="sign.is_public === 1 ? `${sign.sign_name}（公共）` : sign.sign_name"
                :value="sign.sign_name"
              />
            </el-select>
            <span class="form-hint" v-if="!signs.length">暂无已审核通过的签名，请先在「签名」页提交并等待审核通过</span>
          </el-form-item>
          <el-form-item label="短信模板">
            <el-select v-model="form.template_id" placeholder="选择已审核通过的模板" style="max-width: 360px">
              <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.template_name" :value="String(tpl.id)" />
            </el-select>
            <span class="form-hint" v-if="!templates.length">暂无已审核通过的模板，请先在「模板」页提交并等待审核通过</span>
          </el-form-item>
          <el-form-item v-if="selectedTemplate" label="模板内容">
            <span class="form-hint">{{ selectedTemplate.template_content }}</span>
          </el-form-item>
          <el-form-item label="模板参数">
            <el-input
              v-model="form.paramsText"
              placeholder="按模板变量顺序填写，多个参数用英文逗号分隔（无变量可留空）"
              style="max-width: 520px"
            />
          </el-form-item>
          <el-form-item label="手机号">
            <el-input
              v-model="form.phonesText"
              type="textarea"
              :rows="3"
              placeholder="每行一个手机号，也支持英文逗号分隔"
              style="max-width: 520px"
            />
          </el-form-item>
          <el-form-item label="预估计费">
            <span class="form-hint">{{ estimateText }}</span>
          </el-form-item>
          <el-button type="primary" :loading="sending" @click="handleSend">发送</el-button>
        </el-form>
      </div>

      <div class="card">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          最近发送
        </h3>
        <el-table :data="recent" class="table-fill" height="100%" style="width: 100%" v-loading="recentLoading">
          <el-table-column prop="biz_no" label="流水号" width="180" />
          <el-table-column prop="phone_number_set" label="手机号" min-width="150" show-overflow-tooltip />
          <el-table-column prop="sign_name" label="签名" min-width="110" />
          <el-table-column prop="phone_count" label="条数" width="70" />
          <el-table-column label="扣费" width="130">
            <template #default="{ row }">{{ chargeText(row) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag v-if="row.status === 0" type="success">成功</el-tag>
              <el-tag v-else type="danger">失败</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="fail_message" label="失败原因" min-width="140" show-overflow-tooltip />
          <el-table-column prop="created_at" label="发送时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { smsAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const form = ref({ sign_name: '', template_id: '', paramsText: '', phonesText: '' })
const signs = ref<any[]>([])
const templates = ref<any[]>([])
const loading = ref(true)
const sending = ref(false)
const recent = ref<any[]>([])
const recentLoading = ref(false)

const selectedTemplate = computed(() => templates.value.find((tpl) => String(tpl.id) === form.value.template_id) || null)

// 号码解析：按换行、中英文逗号、分号、空白切分并去重
const parsePhones = () =>
  Array.from(
    new Set(
      form.value.phonesText
        .split(/[\s,，;；]+/)
        .map((phone) => phone.trim())
        .filter(Boolean)
    )
  )

const parseParams = () =>
  form.value.paramsText
    .split(',')
    .map((param) => param.trim())
    .filter((param) => param !== '')

// 模板变量占位符（与后端 {%变量%} 一致）
const TEMPLATE_VAR = /\{%[^}]*%\}/g

const renderContent = (content: string, params: string[]) => {
  let index = 0
  return content.replace(TEMPLATE_VAR, (placeholder) => (index < params.length ? params[index++] : placeholder))
}

// 单条计费条数：≤70 字计 1 条，超出按每 67 字递增，最多 15 条（与后端 billedSmsCount 一致）
const billedSegments = (text: string) => {
  const length = [...text].length
  if (length <= 70) return 1
  return Math.min(15, Math.ceil(length / 67))
}

const estimateText = computed(() => {
  if (!selectedTemplate.value) return '选择模板后展示预估条数'
  const params = parseParams()
  const signName = form.value.sign_name || selectedTemplate.value.sign_name || ''
  const segments = billedSegments(`【${signName}】${renderContent(selectedTemplate.value.template_content || '', params)}`)
  const phoneCount = parsePhones().length || 1
  return `单条 ${segments} 计费条数 × ${phoneCount} 个号码 = 预估 ${segments * phoneCount} 条（实际以发送结果为准）`
})

const chargeText = (row: any) => {
  if (row.status === 1) return '-'
  if (row.pay_type === 0) return `资源包 ${row.pack_count || 0} 条`
  return `¥${row.amount}`
}

const loadOptions = async () => {
  loading.value = true
  try {
    const [signRes, tplRes]: any[] = await Promise.all([smsAPI.listSigns({ page: 1, page_size: 100 }), smsAPI.listTemplates({ page: 1, page_size: 100 })])
    signs.value = (signRes?.list || []).filter((sign: any) => sign.status === 2)
    templates.value = (tplRes?.list || []).filter((tpl: any) => tpl.status === 1)
  } catch (error: any) {
    ElMessage.error(error?.message || '加载签名与模板失败')
  } finally {
    loading.value = false
  }
}

const loadRecent = async () => {
  recentLoading.value = true
  try {
    const res: any = await smsAPI.listSendRecords({ page: 1, page_size: 5 })
    recent.value = res?.list || []
  } catch (error) {
    console.error(error)
  } finally {
    recentLoading.value = false
  }
}

const handleSend = async () => {
  if (!form.value.template_id) {
    ElMessage.warning('请选择已审核通过的短信模板')
    return
  }
  const phones = parsePhones()
  if (!phones.length) {
    ElMessage.warning('请输入至少一个手机号')
    return
  }
  sending.value = true
  try {
    await smsAPI.sendSMS({
      phone_number_set: phones,
      template_id: form.value.template_id,
      template_params: parseParams(),
      sign_name: form.value.sign_name
    })
    ElMessage.success('发送成功')
    form.value.phonesText = ''
    form.value.paramsText = ''
    await loadRecent()
  } catch (error: any) {
    ElMessage.error(error?.message || '发送失败，请稍后重试')
    await loadRecent()
  } finally {
    sending.value = false
  }
}

onMounted(() => {
  loadOptions()
  loadRecent()
})
</script>

<style scoped>
.content {
  max-width: 1100px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.send-tip {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.8;
  margin: 0 0 20px;
  padding: 10px 14px;
  background: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.form-hint {
  font-size: 12px;
  color: var(--text-secondary);
  margin-left: 12px;
}
</style>