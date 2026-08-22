<script setup lang="ts">
import { ref } from 'vue'

const tab = ref('inspection')
</script>

<template>
  <div class="content-page">
    <header class="page-header">
      <div><h2>设备保障</h2><p>巡检、维保、校准、故障与停复机</p></div>
      <el-button type="primary">新建工单</el-button>
    </header>
    <el-tabs v-model="tab">
      <el-tab-pane label="巡检" name="inspection" />
      <el-tab-pane label="维保" name="maintenance" />
      <el-tab-pane label="校准" name="calibration" />
      <el-tab-pane label="故障" name="fault" />
    </el-tabs>
    <el-empty description="当前筛选下没有待处理工单" />
  </div>
</template>
