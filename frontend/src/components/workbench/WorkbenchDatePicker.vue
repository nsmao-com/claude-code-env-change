<script setup lang="ts">
import { computed } from 'vue'
import {
  DatePickerRoot, DatePickerAnchor, DatePickerTrigger, DatePickerContent,
  DatePickerCalendar, DatePickerHeader, DatePickerHeading, DatePickerPrev, DatePickerNext,
  DatePickerGrid, DatePickerGridHead, DatePickerGridBody, DatePickerGridRow,
  DatePickerHeadCell, DatePickerCell, DatePickerCellTrigger, type DateValue,
} from 'reka-ui'
import { CalendarDays, ChevronLeft, ChevronRight } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/composables/useI18n'
import { useWorkbench } from '@/composables/useWorkbench'
defineProps<{ label: string; minValue?: DateValue; maxValue?: DateValue; disabled?: boolean }>()
const model = defineModel<DateValue>()
const { locale } = useI18n()
const { tx } = useWorkbench()
const calendarLocale = computed(() => locale.value === 'zh' ? 'zh-CN' : 'en-US')
</script>
<template>
  <DatePickerRoot
    v-model="model"
    :locale="calendarLocale"
    :min-value="minValue"
    :max-value="maxValue"
    :disabled="disabled"
    fixed-weeks
    prevent-deselect
    close-on-select
  >
    <DatePickerAnchor as-child>
      <DatePickerTrigger as-child>
        <Button
          type="button"
          variant="outline"
          class="h-9 w-full justify-start rounded-xl font-normal"
          :disabled="disabled"
          :aria-label="`${label}: ${model?.toString() || tx('未选择', 'Not selected')}`"
        >
          <CalendarDays class="size-4 text-muted-foreground" />
          <span :class="!model ? 'text-muted-foreground' : ''">
            {{ model?.toString() || tx('选择日期', 'Select date') }}
          </span>
        </Button>
      </DatePickerTrigger>
    </DatePickerAnchor>
    <DatePickerContent
      align="start"
      :side-offset="6"
      :collision-padding="12"
      class="z-50 w-72 rounded-xl border border-border bg-popover p-3 text-popover-foreground shadow-lg outline-none"
      :aria-label="label"
    >
      <DatePickerCalendar v-slot="{ grid, weekDays }">
        <DatePickerHeader class="mb-3 flex items-center justify-between gap-2">
          <DatePickerPrev as-child>
            <Button type="button" variant="ghost" size="icon" :aria-label="tx('上个月', 'Previous month')">
              <ChevronLeft />
            </Button>
          </DatePickerPrev>
          <DatePickerHeading class="text-sm font-medium" />
          <DatePickerNext as-child>
            <Button type="button" variant="ghost" size="icon" :aria-label="tx('下个月', 'Next month')">
              <ChevronRight />
            </Button>
          </DatePickerNext>
        </DatePickerHeader>
        <DatePickerGrid v-for="month in grid" :key="month.value.toString()" class="w-full border-collapse">
          <DatePickerGridHead>
            <DatePickerGridRow>
              <DatePickerHeadCell
                v-for="(day, i) in weekDays"
                :key="i"
                class="h-8 text-center text-xs font-normal text-muted-foreground"
              >
                {{ day }}
              </DatePickerHeadCell>
            </DatePickerGridRow>
          </DatePickerGridHead>
          <DatePickerGridBody>
            <DatePickerGridRow v-for="(week, i) in month.rows" :key="i">
              <DatePickerCell v-for="day in week" :key="day.toString()" :date="day" class="p-0.5 text-center">
                <DatePickerCellTrigger
                  :day="day"
                  :month="month.value"
                  class="inline-flex size-8 items-center justify-center rounded-lg text-sm outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring data-[selected]:bg-primary data-[selected]:text-primary-foreground data-[today]:font-bold data-[today]:ring-1 data-[today]:ring-border data-[outside-view]:text-muted-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-30"
                />
              </DatePickerCell>
            </DatePickerGridRow>
          </DatePickerGridBody>
        </DatePickerGrid>
      </DatePickerCalendar>
      <div class="mt-3 border-t border-border pt-2">
        <Button type="button" variant="ghost" class="w-full" :disabled="!model" @click="model = undefined">
          {{ tx('清除日期', 'Clear date') }}
        </Button>
      </div>
    </DatePickerContent>
  </DatePickerRoot>
</template>
