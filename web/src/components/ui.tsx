import { type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react'
import { ChevronDown, Loader2, X } from 'lucide-react'
import { cn } from '../lib/utils'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'subtle'
type ButtonSize = 'sm' | 'md' | 'lg'

const buttonVariants: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 focus-visible:ring-ring',
  secondary: 'bg-secondary text-secondary-foreground border border-border shadow-sm hover:bg-secondary/80 focus-visible:ring-ring',
  ghost: 'bg-transparent text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:ring-ring',
  danger: 'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90 focus-visible:ring-ring',
  subtle: 'bg-primary/10 text-primary hover:bg-primary/15 focus-visible:ring-ring',
}
const buttonSizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-xs gap-1.5',
  md: 'h-10 px-4 text-sm gap-2',
  lg: 'h-11 px-5 text-sm gap-2',
}

export function Button({ className, variant = 'primary', size = 'md', loading, children, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; size?: ButtonSize; loading?: boolean }) {
  return (
    <button
      className={cn('inline-flex items-center justify-center rounded-xl font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50', buttonVariants[variant], buttonSizes[size], className)}
      disabled={loading || props.disabled}
      {...props}
    >
      {loading && <Loader2 className="h-4 w-4 animate-spin" />}
      {children}
    </button>
  )
}

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn('rounded-2xl border border-border bg-card text-card-foreground shadow-sm', className)}>{children}</div>
}

export function Badge({ children, className, tone = 'slate' }: { children: ReactNode; className?: string; tone?: 'slate' | 'green' | 'red' | 'blue' | 'amber' | 'teal' }) {
  const tones = {
    slate: 'bg-muted text-muted-foreground ring-border',
    green: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    red: 'bg-rose-50 text-rose-700 ring-rose-200',
    blue: 'bg-sky-50 text-sky-700 ring-sky-200',
    amber: 'bg-amber-50 text-amber-700 ring-amber-200',
    teal: 'bg-primary/10 text-primary ring-primary/20',
  }
  return <span className={cn('inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-semibold ring-1 ring-inset', tones[tone], className)}>{children}</span>
}

export function StatusBadge({ status }: { status: string }) {
  if (status === 'success') return <Badge tone="green">同步成功</Badge>
  if (status === 'syncing') return <Badge tone="blue"><span className="h-1.5 w-1.5 rounded-full bg-sky-500 pulse-soft" />同步中</Badge>
  if (status === 'failed') return <Badge tone="red">同步失败</Badge>
  if (status === 'pending') return <Badge tone="amber">等待首次同步</Badge>
  return <Badge>{status}</Badge>
}

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn('h-10 w-full rounded-xl border border-input bg-background px-3 text-sm text-foreground shadow-sm outline-none transition placeholder:text-muted-foreground focus:border-ring focus:ring-4 focus:ring-ring/15', className)} {...props} />
}

export function Select({ className, containerClassName, children, ...props }: SelectHTMLAttributes<HTMLSelectElement> & { containerClassName?: string }) {
  return (
    <div className={cn('relative', containerClassName)}>
      <select className={cn('h-10 w-full appearance-none rounded-xl border border-input bg-background px-3 pr-9 text-sm text-foreground shadow-sm outline-none transition focus:border-ring focus:ring-4 focus:ring-ring/15', className)} {...props}>{children}</select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
    </div>
  )
}

export function Spinner({ label = '加载中…' }: { label?: string }) {
  return <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />{label}</div>
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-muted/40 px-6 py-14 text-center">
    <div className="mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-card text-primary shadow-sm ring-1 ring-border">◇</div>
    <h3 className="text-sm font-semibold text-foreground">{title}</h3>
    <p className="mt-1 max-w-md text-xs leading-5 text-muted-foreground">{description}</p>
    {action && <div className="mt-4">{action}</div>}
  </div>
}

export function Modal({ open, title, onClose, children, footer }: { open: boolean; title: string; onClose: () => void; children: ReactNode; footer?: ReactNode }) {
  if (!open) return null
  return <div className="fixed inset-0 z-50 flex items-end justify-center bg-foreground/40 p-0 backdrop-blur-sm sm:items-center sm:p-6" onMouseDown={onClose}>
    <div className="max-h-[92vh] w-full max-w-lg overflow-auto rounded-t-3xl bg-card text-card-foreground shadow-2xl sm:rounded-3xl" onMouseDown={(e) => e.stopPropagation()}>
      <div className="flex items-center justify-between border-b border-border px-5 py-4">
        <h2 className="text-base font-bold text-foreground">{title}</h2>
        <button onClick={onClose} className="rounded-lg p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground"><X className="h-4 w-4" /></button>
      </div>
      <div className="px-5 py-4">{children}</div>
      {footer && <div className="flex justify-end gap-2 border-t border-border px-5 py-4">{footer}</div>}
    </div>
  </div>
}
