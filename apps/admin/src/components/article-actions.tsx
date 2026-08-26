import { EllipsisVertical, Eye, Globe2, Pencil, Trash2, TriangleAlert, UserRoundCheck } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from '@/components/ui/combobox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export type ArticleActionItem = {
  id: string
  headline: string
  public_status: string
  source: string
  category: string | null
}

export type ArticleReviewChange = {
  id: string
  status: string
  category?: string
  reason: string
  quick: boolean
}

export type ArticleCategoryOption = { value: string; label: string }

const reviewStatuses = [
  { value: 'published', label: 'Public' },
  { value: 'held', label: 'Held for review' },
  { value: 'unpublished', label: 'Unpublished' },
]

export function articleCategoryLabel(slug: string) {
  const label = slug.replaceAll('_', ' ').replaceAll('-', ' ')
  return label.replace(/\b\w/g, (character) => character.toUpperCase())
}

function articleStatusLabel(status: string) {
  return status === 'published' ? 'Public' : status.replaceAll('_', ' ')
}

export function ArticleActionsMenu({ article, busy = false, quickPublish = false, onQuickPublish, onEdit, onDelete }: {
  article: ArticleActionItem
  busy?: boolean
  quickPublish?: boolean
  onQuickPublish?: (article: ArticleActionItem) => void
  onEdit: (article: ArticleActionItem) => void
  onDelete: (article: ArticleActionItem) => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Open actions for ${article.headline}`}
            disabled={busy}
          />
        }
      >
        <EllipsisVertical />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        {quickPublish && article.public_status === 'held' && onQuickPublish ? (
          <>
            <DropdownMenuItem onClick={() => onQuickPublish(article)}>
              <Globe2 />
              Make public
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        ) : null}
        <DropdownMenuItem onClick={() => onEdit(article)}>
          <Pencil />
          Edit
        </DropdownMenuItem>
        <DropdownMenuItem nativeButton={false} render={<Link to={`/articles/${article.id}`} />}>
          <Eye />
          View article
        </DropdownMenuItem>
        {article.public_status !== 'removed' ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={() => onDelete(article)}>
              <Trash2 />
              Delete article
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function ArticleReviewDialog({ article, categories, categoriesLoading, open, saving, defaultReason, onOpenChange, onSave }: {
  article: ArticleActionItem
  categories: ArticleCategoryOption[]
  categoriesLoading: boolean
  open: boolean
  saving: boolean
  defaultReason: string
  onOpenChange: (open: boolean) => void
  onSave: (change: ArticleReviewChange) => void
}) {
  const initialCategory = article.category ?? ''
  const initialStatus = article.public_status === 'removed' ? 'unpublished' : article.public_status
  const [status, setStatus] = useState(initialStatus)
  const [category, setCategory] = useState(initialCategory)
  const [reason, setReason] = useState('')
  const items = initialCategory && !categories.some((option) => option.value === initialCategory)
    ? [{ value: initialCategory, label: articleCategoryLabel(initialCategory) }, ...categories]
    : categories
  const selectedCategory = items.find((option) => option.value === category) ?? null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" showCloseButton={!saving}>
        <DialogHeader>
          <DialogTitle>Edit article</DialogTitle>
          <DialogDescription>
            Confirm the category and publication status before saving this editorial decision.
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault()
            if (!category) return
            onSave({
              id: article.id,
              status,
              category,
              reason: reason.trim() || defaultReason,
              quick: false,
            })
          }}
        >
          <div className="rounded-2xl border bg-muted/20 p-4">
            <p className="line-clamp-2 font-medium leading-snug">{article.headline}</p>
            <p className="mt-1 text-xs text-muted-foreground">{article.source}</p>
          </div>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor={`review-status-${article.id}`}>Status</label>
            <Select value={status} onValueChange={(value) => value && setStatus(value)}>
              <SelectTrigger id={`review-status-${article.id}`} className="w-full" aria-label="Article status">
                <SelectValue>{() => reviewStatuses.find((option) => option.value === status)?.label ?? articleStatusLabel(status)}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {reviewStatuses.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <label className="text-sm font-medium">Category</label>
            <Combobox
              items={items}
              value={selectedCategory}
              disabled={categoriesLoading}
              autoHighlight
              isItemEqualToValue={(option, selected) => option.value === selected.value}
              onValueChange={(option) => setCategory(option?.value ?? '')}
            >
              <ComboboxTrigger className="h-9 w-full justify-between border bg-background" aria-label="Article category">
                {selectedCategory?.label ?? (categoriesLoading ? 'Loading categories…' : 'Choose a category')}
              </ComboboxTrigger>
              <ComboboxContent>
                <ComboboxInput placeholder="Search categories…" aria-label="Search categories" />
                <ComboboxEmpty>No category found.</ComboboxEmpty>
                <ComboboxList>
                  {(option) => <ComboboxItem key={option.value} value={option}>{option.label}</ComboboxItem>}
                </ComboboxList>
              </ComboboxContent>
            </Combobox>
            <p className="text-xs text-muted-foreground">Saving confirms this category as a manual editorial decision.</p>
          </div>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor={`review-reason-${article.id}`}>Review note <span className="font-normal text-muted-foreground">(optional)</span></label>
            <textarea
              id={`review-reason-${article.id}`}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder="Add context for the audit record…"
              className="min-h-20 resize-y rounded-2xl border border-input bg-input/30 px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30"
            />
          </div>
          <div className="flex items-start gap-2 rounded-2xl bg-muted/35 p-3 text-xs text-muted-foreground">
            <UserRoundCheck className="mt-0.5 size-4 shrink-0" />
            <p>This decision and its before/after values will be recorded under your signed-in account.</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" disabled={saving} onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={saving || categoriesLoading || !category}>
              {saving ? 'Saving…' : status === 'published' ? 'Save & publish' : 'Save changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function ArticleDeleteDialog({ article, open, deleting, onOpenChange, onConfirm }: {
  article: ArticleActionItem
  open: boolean
  deleting: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (article: ArticleActionItem) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={!deleting}>
        <DialogHeader>
          <div className="mb-1 flex size-10 items-center justify-center rounded-full bg-destructive/10 text-destructive">
            <TriangleAlert className="size-5" />
          </div>
          <DialogTitle>Delete this article?</DialogTitle>
          <DialogDescription>
            This removes the article from public feeds and normal article lists. Its record and audit history are retained.
          </DialogDescription>
        </DialogHeader>
        <div className="rounded-2xl border bg-muted/20 p-4">
          <p className="line-clamp-2 font-medium leading-snug">{article.headline}</p>
          <p className="mt-1 text-xs text-muted-foreground">{article.source}</p>
        </div>
        <DialogFooter>
          <Button variant="outline" disabled={deleting} onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="destructive" disabled={deleting} onClick={() => onConfirm(article)}>
            <Trash2 />
            {deleting ? 'Deleting…' : 'Delete article'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
