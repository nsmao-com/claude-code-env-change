import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as Toggle } from './Toggle.vue'

export const toggleVariants = cva(
  'hover:text-foreground aria-invalid:border-destructive gap-1 rounded-full text-sm font-medium transition-[color,background-color,box-shadow,transform] duration-200 ease-out [&_svg:not([class*=size-])]:size-4 group/toggle hover:bg-muted inline-flex items-center justify-center whitespace-nowrap outline-none disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 data-[state=on]:bg-foreground data-[state=on]:text-background data-[state=on]:hover:bg-foreground/85',
  {
    variants: {
      variant: {
        default: 'bg-transparent',
        outline: 'border border-input/80 bg-transparent hover:bg-muted data-[state=on]:border-transparent',
        pill: 'rounded-full border-0 bg-transparent text-muted-foreground shadow-none hover:bg-transparent hover:text-foreground data-[state=on]:bg-foreground data-[state=on]:text-background data-[state=on]:shadow-sm',
      },
      size: {
        default: 'h-8 min-w-8 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2',
        sm: 'h-7 min-w-7 px-2.5 text-[0.8rem] has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*=size-])]:size-3.5',
        lg: 'h-9 min-w-9 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export type ToggleVariants = VariantProps<typeof toggleVariants>
