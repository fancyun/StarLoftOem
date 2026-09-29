<template>
  <div class="sms-sign-form-container">
    <div class="content">
      <div class="card" v-loading="detailLoading">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><EditPen /></el-icon>
            {{ isEdit ? '修改签名' : '提交签名' }}
          </h3>
          <span class="section-tip" v-if="!isEdit">资质材料并入签名提交，审核通过后可用于短信发送</span>
          <span class="section-tip" v-else>修改将先删除旧签名、再重新报备提交；资质图已回填，未重传则沿用原图</span>
        </div>

        <el-form :model="form" :rules="rules" ref="formRef" label-width="120px" class="sign-form">
          <el-form-item label="签名内容" prop="sign_name">
            <el-input
              v-model="form.sign_name"
              placeholder="如：某某网络（临沂），【】与中文括号将自动转换"
              maxlength="16"
              class="full"
              @blur="form.sign_name = normalizeSignName(form.sign_name)"
            />
          </el-form-item>

          <el-form-item label="资质类型" prop="label">
            <el-select v-model="form.label" class="full">
              <el-option label="营业执照" :value="1" />
              <el-option label="商标" :value="2" />
            </el-select>
            <div class="form-tip">签名归属签名企业自身（他公司报备），需提供下列企业主体与经办人资料</div>
          </el-form-item>

          <el-form-item label="公司名称" prop="company">
            <el-input v-model="form.company" placeholder="企业全称" maxlength="128" class="full" />
          </el-form-item>
          <el-form-item label="法人姓名" prop="legal_person">
            <el-input v-model="form.legal_person" placeholder="法定代表人姓名" maxlength="64" class="full" />
          </el-form-item>
          <el-form-item label="信用代码" prop="credit_code">
            <el-input v-model="form.credit_code" placeholder="统一社会信用代码" maxlength="64" class="full" />
          </el-form-item>
          <el-form-item label="经办人姓名" prop="credit_user_name">
            <el-input v-model="form.credit_user_name" placeholder="经办人姓名" maxlength="64" class="full" />
          </el-form-item>
          <el-form-item label="经办人身份证号" prop="id_card">
            <el-input v-model="form.id_card" placeholder="经办人身份证号码" maxlength="64" class="full" />
          </el-form-item>
          <el-form-item label="经办人手机号" prop="phone">
            <el-input v-model="form.phone" placeholder="经办人手机号" maxlength="32" class="full" />
          </el-form-item>

          <el-form-item label="营业执照" prop="credit_code_url">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.credit_code_url"
              :http-request="(opt: any) => uploadImage(opt, 'credit_code_url')"
              :on-remove="() => clearMaterial('credit_code_url')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
          </el-form-item>

          <el-form-item label="身份证正面" prop="id_card_front">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.id_card_front"
              :http-request="(opt: any) => uploadImage(opt, 'id_card_front')"
              :on-remove="() => clearMaterial('id_card_front')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
          </el-form-item>

          <el-form-item label="身份证反面" prop="id_card_back">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.id_card_back"
              :http-request="(opt: any) => uploadImage(opt, 'id_card_back')"
              :on-remove="() => clearMaterial('id_card_back')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
          </el-form-item>

          <el-form-item v-if="form.label === 2" label="备案截图" prop="screenshot">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.screenshot"
              :http-request="(opt: any) => uploadImage(opt, 'screenshot')"
              :on-remove="() => clearMaterial('screenshot')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
            <div class="form-tip">商标备案证明截图（商标网查询截图）</div>
          </el-form-item>

          <el-form-item label="签名授权书" prop="auth_letter">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.auth_letter"
              :http-request="(opt: any) => uploadImage(opt, 'auth_letter')"
              :on-remove="() => clearMaterial('auth_letter')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
            <div class="form-tip">必填：加盖公章/签章的《短信签名授权书》扫描件（下载模板填写盖章后上传）</div>
          </el-form-item>

          <el-form-item label="意愿承诺函" prop="sx_commits">
            <el-upload
              class="upload-item"
              accept=".jpg,.jpeg,.png"
              list-type="picture-card"
              :limit="1"
              :file-list="materialFileLists.sx_commits"
              :http-request="(opt: any) => uploadImage(opt, 'sx_commits')"
              :on-remove="() => clearMaterial('sx_commits')"
              :on-exceed="() => ElMessage.warning('请先点击图片上的删除，再选择新图片')"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
            <div class="form-tip">选填（营销短信须上传）；未上传时以营业执照图片作为意愿承诺函报备</div>
          </el-form-item>

          <el-form-item label="模板下载">
            <div class="tpl-download">
              <a href="/templates/sms-commitment.docx" target="_blank" rel="noopener" class="agree-link">下载《用户接收意愿承诺函》模板</a>
              <a href="/templates/sms-authorization.docx" target="_blank" rel="noopener" class="agree-link">下载《短信签名授权书》模板</a>
            </div>
          </el-form-item>

          <el-form-item>
            <el-button size="large" @click="goBack">返回</el-button>
            <el-button type="primary" size="large" class="submit-btn" :loading="loading" @click="handleSubmit">
              {{ isEdit ? '保存修改' : '提交审核' }}
            </el-button>
            <p class="sms-legal">
              提交即表示您已阅读并同意
              <a :href="`${siteBase('portal')}/sms-rules`" target="_blank" rel="noopener" class="agree-link">《短信服务使用规范》</a>
            </p>
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { smsAPI } from '@/api'
import { siteBase } from '@/utils/promotion'
import { loadAuthImage, compressImageIfNeeded } from '@/utils/image'

const route = useRoute()
const router = useRouter()

const editingId = computed(() => Number(route.params.id) || 0)
const isEdit = computed(() => editingId.value > 0)

const loading = ref(false)
const detailLoading = ref(false)
const formRef = ref()

// 资质图字段与中文名（修改态把已有资质图回填到对应上传框）
const materialFields = [
  { key: 'credit_code_url', label: '营业执照' },
  { key: 'id_card_front', label: '身份证正面' },
  { key: 'id_card_back', label: '身份证反面' },
  { key: 'screenshot', label: '备案截图' },
  { key: 'auth_letter', label: '签名授权书' },
  { key: 'sx_commits', label: '意愿承诺函' }
]
// 各上传框的文件列表（修改态填充已有图片；数组引用保持稳定，避免 el-upload 反复重置）
const materialFileLists = reactive<Record<string, any[]>>({
  credit_code_url: [],
  id_card_front: [],
  id_card_back: [],
  screenshot: [],
  auth_letter: [],
  sx_commits: []
})

const form = reactive({
  sign_name: '',
  label: 1,
  company: '',
  legal_person: '',
  credit_code: '',
  credit_user_name: '',
  id_card: '',
  phone: '',
  credit_code_url: '',
  id_card_front: '',
  id_card_back: '',
  sx_commits: '',
  auth_letter: '',
  screenshot: ''
})

// 签名内容允许中文/英文/数字与中英文圆括号：上游不允许含表情等符号（下发时自动补充【】）
const signNameValid = (v: string) => /^[\u4e00-\u9fa5A-Za-z0-9()（）]+$/.test(String(v || ''))

// 去【】中括号（下发时上游会自动补【】）、中文圆括号统一替换为英文圆括号
const normalizeSignName = (v: string) =>
  String(v || '').replace(/[【】]/g, '').replace(/（/g, '(').replace(/）/g, ')')

const rules: any = {
  sign_name: [
    { required: true, message: '请输入签名内容', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, cb: any) =>
        !value || signNameValid(normalizeSignName(value)) ? cb() : cb(new Error('签名内容只能包含中文、英文、数字与括号，不能含表情等符号')),
      trigger: 'blur'
    }
  ],
  label: [{ required: true, message: '请选择资质类型', trigger: 'change' }],
  credit_code_url: [{ required: true, message: '请上传营业执照图片', trigger: 'change' }],
  id_card_front: [{ required: true, message: '请上传身份证正面图片', trigger: 'change' }],
  id_card_back: [{ required: true, message: '请上传身份证反面图片', trigger: 'change' }],
  auth_letter: [{ required: true, message: '请上传签名授权书', trigger: 'change' }],
  company: [{ required: true, message: '请填写公司名称', trigger: 'blur' }],
  legal_person: [{ required: true, message: '请填写法人姓名', trigger: 'blur' }],
  credit_code: [{ required: true, message: '请填写统一社会信用代码', trigger: 'blur' }],
  credit_user_name: [{ required: true, message: '请填写经办人姓名', trigger: 'blur' }],
  id_card: [{ required: true, message: '请填写经办人身份证号', trigger: 'blur' }],
  phone: [{ required: true, message: '请填写经办人手机号', trigger: 'blur' }]
}

// 上传图片到平台，成功后回填对应表单字段为图片 URL（仅字符串 URL 字段）
const uploadImage = async (opt: any, field: 'credit_code_url' | 'id_card_front' | 'id_card_back' | 'sx_commits' | 'auth_letter' | 'screenshot') => {
  try {
    // 超过阈值的大图先在前端压缩，避免超出请求体限制被网关拒绝
    const file = await compressImageIfNeeded(opt.file)
    const res: any = await smsAPI.upload(file)
    // 清洗图片地址，防止两端混入反引号/空白导致上游报「材料不能为空」
    form[field] = String(res.url || '').replace(/[`\u0060\uFF40]/g, '').trim()
    opt.onSuccess(res)
  } catch (e) {
    opt.onError(e)
    ElMessage.error('图片上传失败，请重试')
  }
}

// 释放文件列表里的 object URL（仅回显图片为 blob:，不影响表单里的图片 URL）
const releaseFileList = (files?: any[]) => {
  files?.forEach((f) => {
    const url = String(f?.url || '')
    if (url.startsWith('blob:')) URL.revokeObjectURL(url)
  })
}

// 清空某个上传框：用户点删除时同步清掉表单里的图片 URL 与文件列表
const clearMaterial = (field: string) => {
  releaseFileList(materialFileLists[field])
  materialFileLists[field] = []
  ;(form as any)[field] = ''
}

// 释放回显用 object URL 并清空各上传框文件列表（离开页面时调用）
const resetFileLists = () => {
  Object.keys(materialFileLists).forEach((key) => {
    releaseFileList(materialFileLists[key])
    materialFileLists[key] = []
  })
}

// 修改态回显已有资质图：图片受 UploadAuth 保护，需按归属带登录态拉取后填入对应上传框
const loadPreviews = async () => {
  resetFileLists()
  for (const f of materialFields) {
    const url = String((form as any)[f.key] || '')
    if (!url) continue
    try {
      const preview = await loadAuthImage(url)
      if (preview) materialFileLists[f.key] = [{ name: f.label, url: preview, status: 'success' }]
    } catch {
      // 单张图读取失败不影响表单：原图地址仍在表单中，可直接提交
    }
  }
}

// 修改态加载签名详情并回填表单（详情按归属返回，非本人签名取不到）
const loadDetail = async () => {
  if (!isEdit.value) return
  detailLoading.value = true
  try {
    const detail: any = await smsAPI.getSign(editingId.value)
    const d = detail?.data ?? detail
    Object.assign(form, {
      sign_name: d?.sign_name || '',
      label: d?.label || 1,
      company: d?.company || '',
      legal_person: d?.legal_person || '',
      credit_code: d?.credit_code || '',
      credit_user_name: d?.credit_user_name || '',
      id_card: d?.id_card || '',
      phone: d?.phone || '',
      credit_code_url: d?.credit_code_url || '',
      id_card_front: d?.id_card_front || '',
      id_card_back: d?.id_card_back || '',
      sx_commits: d?.sx_commits || '',
      auth_letter: d?.auth_letter || '',
      screenshot: d?.screenshot || ''
    })
    await loadPreviews()
  } catch (error) {
    ElMessage.error((error as Error)?.message || '签名详情加载失败')
  } finally {
    detailLoading.value = false
  }
}

const goBack = () => {
  router.push('/sms/sign')
}

const handleSubmit = async () => {
  form.sign_name = normalizeSignName(form.sign_name)
  await formRef.value.validate()
  if (form.label === 2 && !form.screenshot) {
    ElMessage.error('请上传商标备案截图')
    return
  }
  loading.value = true
  try {
    if (isEdit.value) {
      await smsAPI.updateSign(editingId.value, { ...form })
      ElMessage.success('修改已提交，等待重新审核')
    } else {
      await smsAPI.submitSign({ ...form })
      ElMessage.success('签名提交成功，请等待审核')
    }
    router.push('/sms/sign')
  } catch (error) {
    ElMessage.error((error as Error)?.message || '提交失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadDetail()
})

onUnmounted(() => {
  resetFileLists()
})
</script>

<style scoped>
.sms-sign-form-container {
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

.sign-form {
  margin-top: 8px;
}

.full {
  width: 100%;
}

.submit-btn {
  width: 200px;
}

.sms-legal {
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-muted);
}

.form-tip {
  width: 100%;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.tpl-download {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}

.tpl-download .agree-link {
  color: var(--color-primary);
  text-decoration: none;
}

.sms-legal .agree-link {
  color: var(--color-primary);
  text-decoration: none;
}
</style>