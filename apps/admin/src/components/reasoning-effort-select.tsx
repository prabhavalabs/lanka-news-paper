import type { ReasoningEffort } from '@snap/api-client'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export function ReasoningEffortSelect({ value, onChange, efforts = ['low', 'medium', 'high'] }: { value: ReasoningEffort | ''; onChange: (value: ReasoningEffort | '') => void; efforts?: ReasoningEffort[] }) {
  return <div className="space-y-2"><p className="text-xs font-medium text-muted-foreground">Reasoning effort</p><Select value={value || 'default'} onValueChange={(next) => onChange(next === 'default' ? '' : next as ReasoningEffort)}><SelectTrigger aria-label="Reasoning effort" className="w-full"><SelectValue>{() => value || 'Model default'}</SelectValue></SelectTrigger><SelectContent><SelectItem value="default">Model default</SelectItem>{efforts.map((effort) => <SelectItem key={effort} value={effort}>{effort}</SelectItem>)}</SelectContent></Select></div>
}
