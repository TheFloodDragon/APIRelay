<script setup>
import Modal from './Modal.vue'
import { useConfirmState } from '../composables/useConfirm'

const { confirmState, settleConfirm } = useConfirmState()
</script>

<template>
  <Modal
    :open="confirmState.open"
    :title="confirmState.title"
    width="max-w-md"
    @close="settleConfirm(false)"
  >
    <div class="confirm-message">
      <span class="confirm-mark" aria-hidden="true">!</span>
      <p class="whitespace-pre-line text-sm leading-6 text-soft">{{ confirmState.message }}</p>
    </div>
    <template #footer>
      <button class="btn" type="button" @click="settleConfirm(false)">取消</button>
      <button
        class="btn"
        :class="confirmState.tone === 'danger' ? 'btn-danger' : 'btn-primary'"
        type="button"
        data-autofocus
        @click="settleConfirm(true)"
      >
        {{ confirmState.confirmLabel }}
      </button>
    </template>
  </Modal>
</template>
