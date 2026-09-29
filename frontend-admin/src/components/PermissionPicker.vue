<template>
  <div class="perm-picker">
    <el-checkbox
      class="perm-all"
      :model-value="hasAll"
      @change="(v: CheckboxValueType) => onAllChange(v)"
    >
      全部权限（含后续新增页面）
    </el-checkbox>

    <div v-for="group in groups" :key="group.name" class="perm-group">
      <div class="perm-group-head">
        <span class="perm-group-title">{{ group.name }}</span>
        <el-checkbox
          :model-value="groupChecked(group.wildcard)"
          :disabled="hasAll"
          @change="(v: CheckboxValueType) => onGroupChange(group.wildcard, v)"
        >
          全部（读写）
        </el-checkbox>
      </div>
      <div class="perm-modules">
        <div v-for="item in group.modules" :key="item.code" class="perm-module">
          <span class="perm-module-name">{{ item.label }}</span>
          <div class="perm-module-ops">
            <el-checkbox
              :model-value="readChecked(item.code)"
              :disabled="readDisabled(item.code)"
              @change="(v: CheckboxValueType) => onTokenChange(item.code, v)"
            >
              查看
            </el-checkbox>
            <el-checkbox
              v-if="item.writable"
              :model-value="writeChecked(item.code)"
              :disabled="writeDisabled(item.code)"
              @change="(v: CheckboxValueType) => onTokenChange(writeCode(item.code), v)"
            >
              修改
            </el-checkbox>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  GROUP_WILDCARDS,
  PERMISSION_ALL,
  PERMISSION_CATALOG,
  groupOf,
  writeCode
} from '@/permissions'

// CheckboxValueType 为 element-plus 复选框 change 事件的值类型（string | number | boolean）
type CheckboxValueType = string | number | boolean

// 权限勾选器：每个模块分「查看」（读码）/「修改」（写码，仅可写模块）。
// 规则：勾选分组「全部（读写）」或「全部权限」时，其覆盖项以只读勾选态展示；
// 「修改」隐含「查看」；保存时由后端再归一化（去冗余、三组通配齐备收敛为 all）。
const props = defineProps<{ modelValue: string[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: string[]): void }>()

const groups = computed(() => {
  const list: {
    name: string
    wildcard: string
    modules: { code: string; label: string; writable: boolean }[]
  }[] = []
  for (const spec of PERMISSION_CATALOG) {
    let group = list.find((g) => g.name === spec.group)
    if (!group) {
      group = { name: spec.group, wildcard: GROUP_WILDCARDS[spec.group] || '', modules: [] }
      list.push(group)
    }
    group.modules.push({ code: spec.code, label: spec.label, writable: spec.writable })
  }
  return list
})

const hasAll = computed(() => props.modelValue.includes(PERMISSION_ALL))
const groupChecked = (wildcard: string) => hasAll.value || props.modelValue.includes(wildcard)

// 模块「查看」：显式勾选、或被同模块「修改」、或被 all / 分组通配码覆盖，均视为已勾选
const readChecked = (code: string) =>
  hasAll.value ||
  props.modelValue.includes(code) ||
  props.modelValue.includes(writeCode(code)) ||
  props.modelValue.includes(groupOf(code))

// 模块「修改」：显式勾选，或被 all / 分组通配码覆盖
const writeChecked = (code: string) =>
  hasAll.value || props.modelValue.includes(writeCode(code)) || props.modelValue.includes(groupOf(code))

// 「查看」在已被同模块「修改」或通配覆盖时不可单独取消
const readDisabled = (code: string) =>
  hasAll.value || props.modelValue.includes(groupOf(code)) || props.modelValue.includes(writeCode(code))

const writeDisabled = (code: string) => hasAll.value || props.modelValue.includes(groupOf(code))

const onAllChange = (value: CheckboxValueType) => {
  emit('update:modelValue', value ? [PERMISSION_ALL] : [])
}

const onGroupChange = (wildcard: string, value: CheckboxValueType) => {
  const list = [...props.modelValue]
  const index = list.indexOf(wildcard)
  if (value) {
    if (index < 0) list.push(wildcard)
    // 通配已覆盖本组，组内具体码不再冗余保留
    for (let i = list.length - 1; i >= 0; i--) {
      if (list[i] !== wildcard && groupOf(list[i]) === wildcard) list.splice(i, 1)
    }
  } else if (index >= 0) {
    list.splice(index, 1)
  }
  emit('update:modelValue', list)
}

const onTokenChange = (token: string, value: CheckboxValueType) => {
  const list = [...props.modelValue]
  const index = list.indexOf(token)
  if (value && index < 0) list.push(token)
  if (!value && index >= 0) list.splice(index, 1)
  emit('update:modelValue', list)
}
</script>

<style scoped>
.perm-picker {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.perm-all {
  font-weight: 600;
}

.perm-group {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
}

.perm-group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 2px;
}

.perm-group-title {
  font-size: 13px;
  color: var(--text-secondary);
}

.perm-modules {
  display: flex;
  flex-direction: column;
}

.perm-module {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 2px 0;
}

.perm-module-name {
  font-size: 13px;
  color: var(--text-primary);
}

.perm-module-ops {
  display: flex;
  align-items: center;
  gap: 12px;
}

.perm-module-ops :deep(.el-checkbox) {
  margin-right: 0;
}
</style>