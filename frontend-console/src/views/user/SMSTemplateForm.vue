<template>
  <div class="sms-template-form-container">
    <div class="content">
      <div class="card" v-loading="detailLoading">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><EditPen /></el-icon>
            {{ isEdit ? '修改模板' : '提交模板' }}
          </h3>
          <span class="section-tip" v-if="!isEdit">提交后{{ needsPlatformReview ? '先经平台审核，通过后再报备上游' : '由上游审核' }}，通过后可用于短信发送</span>
          <span class="section-tip" v-else>修改将按新内容与绑定签名重新报备，需重新审核</span>
        </div>

        <el-form :model="form" :rules="rules" ref="formRef" label-width="110px" class="template-form">
          <el-form-item label="模板名称" prop="template_name">
            <el-input v-model="form.template_name" placeholder="如：登录验证码通知" maxlength="64" class="full" />
          </el-form-item>
          <el-form-item label="模板类型" prop="template_type">
            <el-select v-model="form.template_type" placeholder="选择模板类型" class="full">
              <el-option label="验证码短信" value="verify" />
              <el-option label="通知短信" value="notify" />
              <el-option label="营销短信" value="marketing" />
            </el-select>
            <div class="form-tip">营销短信由营销资源包扣量（与验证码/通知互不通用），内容须含退订方式且需用户接收意愿承诺函</div>
          </el-form-item>
          <el-form-item label="关联签名" prop="sign_id">
            <el-select v-model="form.sign_id" placeholder="选择已审核签名" class="full">
              <el-option
                v-for="sign in approvedSigns"
                :key="sign.id"
                :label="sign.is_public === 1 ? `${sign.sign_name}（公共）` : sign.sign_name"
                :value="sign.id"
              />
            </el-select>
            <div class="form-tip">上游报备模板须绑定签名，仅可选择已审核通过的签名；标注「公共」的签名由平台提供，所有账号均可使用</div>
            <div v-if="needsPlatformReview" class="form-tip">
              所选「公共签名」由平台提供，需先经平台审核，通过后才报备上游
            </div>
          </el-form-item>
          <el-form-item label="模板内容" prop="template_content">
            <el-input
              v-model="form.template_content"
              type="textarea"
              :rows="4"
              placeholder="模板内容，变量用 {%变量1%}、{%变量2%} 占位（联麓模板格式），如：您的验证码为{%变量1%}，{%变量2%}分钟内有效。"
            />
          </el-form-item>
          <el-form-item>
            <el-button size="large" @click="goBack">返回</el-button>
            <el-button type="primary" size="large" class="submit-btn" :loading="loading" @click="handleSubmit">
              {{ isEdit ? '保存修改' : '提交审核' }}
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { smsAPI } from '@/api'

const route = useRoute()
const router = useRouter()

const editingId = computed(() => Number(route.params.id) || 0)
const isEdit = computed(() => editingId.value > 0)

const loading = ref(false)
const detailLoading = ref(false)
const formRef = ref()
const approvedSigns = ref<any[]>([])

const form = reactive({
  template_name: '',
  template_type: '',
  sign_id: undefined as number | undefined,
  template_content: ''
})

const rules: any = {
  template_name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  template_type: [{ required: true, message: '请选择模板类型', trigger: 'change' }],
  sign_id: [{ required: true, message: '请选择关联签名', trigger: 'change' }],
  template_content: [{ required: true, message: '请输入模板内容', trigger: 'blur' }]
}

// 所选为公共签名（is_public=1）时，提交/修改需先经平台审核，通过后才报备上游
const needsPlatformReview = computed(() => {
  const sign = approvedSigns.value.find((s: any) => s.id === form.sign_id)
  return sign?.is_public === 1
})

const loadSigns = async () => {
  try {
    const res = await smsAPI.listSigns({ page: 1, page_size: 100 })
    approvedSigns.value = (res.list || []).filter((s: any) => s.status === 2)
  } catch (error) {
    console.error(error)
  }
}

// 修改态加载模板详情并回填表单（详情按归属返回，非本人模板取不到）
const loadDetail = async () => {
  if (!isEdit.value) return
  detailLoading.value = true
  try {
    const detail: any = await smsAPI.getTemplate(editingId.value)
    const d = detail?.data ?? detail
    const selectable = approvedSigns.value.some((s: any) => s.id === d?.sign_id)
    Object.assign(form, {
      template_name: d?.template_name || '',
      template_type: d?.template_type || '',
      sign_id: selectable ? d?.sign_id : undefined,
      template_content: d?.template_content || ''
    })
  } catch (error) {
    ElMessage.error((error as Error)?.message || '模板详情加载失败')
  } finally {
    detailLoading.value = false
  }
}

const goBack = () => {
  router.push('/sms/template')
}

const handleSubmit = async () => {
  await formRef.value.validate()
  loading.value = true
  try {
    if (isEdit.value) {
      await smsAPI.updateTemplate(editingId.value, { ...form })
      ElMessage.success('修改已提交，等待重新审核')
    } else {
      await smsAPI.createTemplate({ ...form })
      ElMessage.success('模板提交成功，请等待审核')
    }
    router.push('/sms/template')
  } catch (error) {
    ElMessage.error((error as Error)?.message || '提交失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadSigns()
  loadDetail()
})
</script>

<style scoped>
.sms-template-form-container {
  min-height: 100%;
}

.content {
  max-width: 960px;
  margin: 0 auto;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.section-tip {
  font-size: 13px;
  color: var(--text-muted);
}

.template-form {
  margin-top: 8px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
}

.full {
  width: 100%;
}

.submit-btn {
  width: 200px;
}
</style>