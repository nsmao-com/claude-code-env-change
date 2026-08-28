<template>
  <AlertDialog :open="isOpen" @update:open="onOpenChange">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        <AlertDialogDescription>{{ message }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <Button type="button" variant="outline" @click="cancel">取消</Button>
        <Button
          type="button"
          :variant="confirmType === 'danger' ? 'destructive' : 'default'"
          @click="confirm"
        >
          确定
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script setup lang="ts">
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

const { isOpen, title, message, confirmType, confirm, cancel } = useConfirm()

function onOpenChange(open: boolean) {
  if (!open) cancel()
}
</script>
