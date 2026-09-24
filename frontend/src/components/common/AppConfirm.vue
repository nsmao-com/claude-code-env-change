<template>
  <AlertDialog :open="isOpen" @update:open="onOpenChange">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        <AlertDialogDescription>{{ message }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <Button type="button" variant="outline" @click="cancel">{{ t('common.cancel') }}</Button>
        <Button
          type="button"
          :variant="confirmType === 'danger' ? 'destructive' : 'default'"
          @click="confirm"
        >
          {{ t('common.confirm') }}
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { useConfirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const { t } = useI18n()

const { isOpen, title, message, confirmType, confirm, cancel } = useConfirm()

function onOpenChange(open: boolean) {
  if (!open) cancel()
}
</script>
