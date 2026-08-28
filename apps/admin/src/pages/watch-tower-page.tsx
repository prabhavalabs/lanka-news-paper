import {
  createClient,
  type LlmModel,
  type WatchTowerConversation,
  type WatchTowerMessage,
  type WatchTowerThread,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowUp,
  Bot,
  BookOpenText,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Clock3,
  Database,
  ExternalLink,
  History,
  LoaderCircle,
  MessageSquareText,
  Plus,
  Radar,
  Search,
  Sparkles,
  Telescope,
  Trash2,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { RichArticleContent } from '@/components/rich-article-content'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

const client = createClient()
const dateTime = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
const relativeTime = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })

const suggestedQuestions = [
  'What happened in Sri Lanka in the last 24 hours?',
  'What are the biggest political stories this week?',
  'What economic issues are news sources covering today?',
  'Which current stories have the widest source coverage?',
]

export function WatchTowerPage() {
  const queryClient = useQueryClient()
  const [selectedThreadID, setSelectedThreadID] = useState<string | null>(null)
  const [prompt, setPrompt] = useState('')
  const [pendingQuestion, setPendingQuestion] = useState('')
  const [historyOpen, setHistoryOpen] = useState(false)
  const [modelPickerOpen, setModelPickerOpen] = useState(false)
  const [modelSearch, setModelSearch] = useState('')
  const [selectedModel, setSelectedModel] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<WatchTowerThread | null>(null)
  const messageEnd = useRef<HTMLDivElement>(null)
  const user = queryClient.getQueryData<{ role: string }>(['me'])

  const threads = useQuery({
    queryKey: ['watch-tower-threads'],
    queryFn: () => client.watchTowerThreads(),
    staleTime: 10_000,
  })
  const conversation = useQuery({
    queryKey: ['watch-tower-conversation', selectedThreadID],
    queryFn: () => client.watchTowerConversation(selectedThreadID!),
    enabled: Boolean(selectedThreadID),
    staleTime: 10_000,
  })
  const provider = useQuery({
    queryKey: ['llm-provider'],
    queryFn: () => client.llmProvider(),
    staleTime: 60_000,
    enabled: user?.role === 'administrator',
  })
  const models = useQuery({
    queryKey: ['llm-models'],
    queryFn: () => client.llmModels(),
    staleTime: 60_000,
    enabled: user?.role === 'administrator',
  })
  const profiles = useQuery({
    queryKey: ['llm-profiles'],
    queryFn: () => client.llmProfiles(),
    staleTime: 60_000,
    enabled: user?.role === 'administrator',
  })

  const watchTowerProfiles = (profiles.data?.items ?? []).filter((profile) =>
    profile.task === 'watch_tower_retrieval' || profile.task === 'watch_tower_answer')
  const retrievalModel = watchTowerProfiles.find((profile) => profile.task === 'watch_tower_retrieval')?.model ?? ''
  const answerModel = watchTowerProfiles.find((profile) => profile.task === 'watch_tower_answer')?.model ?? ''
  const configuredModel = answerModel || retrievalModel
  const profilesDiffer = Boolean(retrievalModel && answerModel && retrievalModel !== answerModel)
  const compatibleModels = useMemo(() => (models.data?.items ?? []).filter((model) =>
    model.compatible_tasks.includes('watch_tower_retrieval') && model.compatible_tasks.includes('watch_tower_answer')),
  [models.data?.items])
  const currentModel = compatibleModels.find((model) => model.id === configuredModel)
  const searchTerm = modelSearch.trim().toLowerCase()
  const visibleModels = useMemo(() => compatibleModels.filter((model) =>
    !searchTerm || model.name.toLowerCase().includes(searchTerm) || model.id.toLowerCase().includes(searchTerm)),
  [compatibleModels, searchTerm])

  const ask = useMutation({
    mutationFn: async (question: string) => {
      let threadID = selectedThreadID
      if (!threadID) {
        const thread = await client.createWatchTowerThread(question)
        threadID = thread.id
        setSelectedThreadID(thread.id)
      }
      return client.askWatchTower(threadID, question)
    },
    onMutate: (question) => {
      setPendingQuestion(question)
      setPrompt('')
    },
    onSuccess: async (exchange) => {
      setSelectedThreadID(exchange.thread.id)
      queryClient.setQueryData<WatchTowerConversation>(
        ['watch-tower-conversation', exchange.thread.id],
        (current) => ({
          thread: exchange.thread,
          messages: [...(current?.messages ?? []), exchange.user, exchange.assistant],
        }),
      )
      await queryClient.invalidateQueries({ queryKey: ['watch-tower-threads'] })
    },
    onError: (_error, question) => {
      setPrompt(question)
      toast.error('Watch Tower could not complete that question. Please try again.')
    },
    onSettled: () => setPendingQuestion(''),
  })

  const removeThread = useMutation({
    mutationFn: (id: string) => client.deleteWatchTowerThread(id),
    onSuccess: async (_, id) => {
      queryClient.removeQueries({ queryKey: ['watch-tower-conversation', id] })
      if (selectedThreadID === id) setSelectedThreadID(null)
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['watch-tower-threads'] })
      toast.success('Conversation deleted')
    },
    onError: () => toast.error('Could not delete this conversation'),
  })

  const updateModel = useMutation({
    mutationFn: (model: string) => client.updateWatchTowerModel({ provider_id: 'openrouter', model }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['llm-profiles'] }),
        queryClient.invalidateQueries({ queryKey: ['agent-workflows'] }),
      ])
      setModelPickerOpen(false)
      setModelSearch('')
      toast.success('Watch Tower model updated for retrieval and answers')
    },
    onError: () => toast.error('Could not update the Watch Tower model'),
  })

  useEffect(() => {
    if (!pendingQuestion && !(conversation.data?.messages.length)) return
    messageEnd.current?.scrollIntoView({ block: 'end', behavior: 'smooth' })
  }, [conversation.data?.messages.length, pendingQuestion])

  const submitQuestion = (question = prompt) => {
    const value = question.trim()
    if (!value || ask.isPending) return
    ask.mutate(value)
  }

  const chooseThread = (id: string) => {
    setSelectedThreadID(id)
    setHistoryOpen(false)
  }

  const startNew = () => {
    setSelectedThreadID(null)
    setPrompt('')
    setHistoryOpen(false)
  }

  const openModelPicker = () => {
    setSelectedModel(configuredModel)
    setModelSearch('')
    setModelPickerOpen(true)
  }

  return (
    <section className="watch-tower-page flex min-h-[34rem] min-w-0 flex-col gap-4">
      <div className="flex shrink-0 flex-wrap items-end justify-between gap-3">
        <div className="min-w-0">
          <Badge variant="outline" className="mb-2"><Radar /> Newsroom intelligence</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Watch Tower</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            Ask what is happening in Sri Lanka and inspect the newsroom evidence behind every answer.
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {user?.role === 'administrator' ? (
            <Button
              variant="outline"
              className="max-w-[17rem] justify-between"
              onClick={openModelPicker}
              disabled={ask.isPending}
              aria-label={`Change Watch Tower model. Current model: ${currentModel?.name ?? configuredModel ?? 'not configured'}`}
            >
              <Bot />
              <span className="hidden text-muted-foreground sm:inline">Model</span>
              <span className="truncate font-medium">{currentModel?.name ?? configuredModel ?? 'Choose model'}</span>
              <ChevronDown />
            </Button>
          ) : null}
          <div className="watch-tower-mobile-history">
            <Button variant="outline" onClick={() => setHistoryOpen(true)}>
              <History /> History
            </Button>
          </div>
        </div>
      </div>

      <Card className="min-h-0 flex-1 gap-0 rounded-3xl py-0">
        <div className="flex min-h-0 flex-1">
          <aside className="hidden w-72 shrink-0 flex-col border-r lg:flex">
            <ThreadHistoryHeader onNew={startNew} />
            <ScrollArea className="min-h-0 flex-1">
              <ThreadHistory
                threads={threads.data?.items ?? []}
                selectedID={selectedThreadID}
                loading={threads.isPending}
                onSelect={chooseThread}
                onDelete={setDeleteTarget}
              />
            </ScrollArea>
          </aside>

          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex h-14 shrink-0 items-center gap-3 border-b px-4 sm:px-6">
              <div className="flex size-8 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Telescope className="size-4" />
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{conversation.data?.thread.title ?? 'New intelligence brief'}</p>
                <p className="truncate text-xs text-muted-foreground">Grounded in your captured Sri Lankan news corpus</p>
              </div>
              {conversation.data?.messages.length ? (
                <Badge variant="secondary" className="ml-auto hidden sm:flex">
                  <MessageSquareText /> {conversation.data.messages.length} messages
                </Badge>
              ) : null}
            </div>

            <ScrollArea className="min-h-0 flex-1">
              <div className="mx-auto flex min-h-full w-full max-w-4xl flex-col px-4 py-6 sm:px-8 sm:py-8">
                {selectedThreadID && conversation.isPending ? (
                  <ConversationSkeleton />
                ) : conversation.data?.messages.length ? (
                  <div className="flex flex-col gap-7">
                    {conversation.data.messages.map((message) => (
                      <ChatMessage key={message.id} message={message} onSuggestion={submitQuestion} disabled={ask.isPending} />
                    ))}
                  </div>
                ) : (
                  <Welcome onQuestion={submitQuestion} disabled={ask.isPending} />
                )}

                {pendingQuestion ? (
                  <PendingAnswer question={pendingQuestion} />
                ) : null}
                <div ref={messageEnd} />
              </div>
            </ScrollArea>

            <form
              className="shrink-0 border-t bg-card/95 p-3 backdrop-blur sm:p-4"
              onSubmit={(event) => {
                event.preventDefault()
                submitQuestion()
              }}
            >
              <div className="mx-auto flex max-w-4xl items-end gap-2 rounded-2xl border bg-background p-2 shadow-sm focus-within:ring-2 focus-within:ring-ring/30">
                <Textarea
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && !event.shiftKey) {
                      event.preventDefault()
                      submitQuestion()
                    }
                  }}
                  maxLength={4000}
                  rows={1}
                  placeholder="Ask Watch Tower what is happening…"
                  className="max-h-36 min-h-10 resize-none border-0 bg-transparent px-3 py-2 shadow-none focus-visible:ring-0 dark:bg-transparent"
                  disabled={ask.isPending}
                />
                <Button type="submit" size="icon" className="mb-0.5 shrink-0 rounded-xl" disabled={!prompt.trim() || ask.isPending}>
                  {ask.isPending ? <LoaderCircle className="animate-spin" /> : <ArrowUp />}
                  <span className="sr-only">Ask Watch Tower</span>
                </Button>
              </div>
              <p className="mx-auto mt-2 max-w-4xl text-center text-[11px] text-muted-foreground">
                Watch Tower answers from captured newsroom data. Open citations to verify important details.
              </p>
            </form>
          </div>
        </div>
      </Card>

      <Sheet open={historyOpen} onOpenChange={setHistoryOpen}>
        <SheetContent side="left" className="w-[88vw] max-w-sm p-0">
          <SheetHeader className="border-b pr-14">
            <SheetTitle>Conversation history</SheetTitle>
            <SheetDescription>Return to a saved Watch Tower investigation.</SheetDescription>
          </SheetHeader>
          <ThreadHistoryHeader onNew={startNew} />
          <ScrollArea className="min-h-0 flex-1">
            <ThreadHistory
              threads={threads.data?.items ?? []}
              selectedID={selectedThreadID}
              loading={threads.isPending}
              onSelect={chooseThread}
              onDelete={setDeleteTarget}
            />
          </ScrollArea>
        </SheetContent>
      </Sheet>

      <Sheet open={modelPickerOpen} onOpenChange={setModelPickerOpen}>
        <SheetContent side="right" style={{ width: '100%', maxWidth: '38rem' }}>
          <SheetHeader className="border-b pr-14">
            <SheetTitle>Watch Tower model</SheetTitle>
            <SheetDescription>
              One model powers both evidence retrieval and cited answer generation. Changes apply to the next question.
            </SheetDescription>
          </SheetHeader>

          <div className="space-y-4 border-b px-6 pb-5">
            <div className="flex items-center justify-between gap-3 rounded-xl border bg-muted/20 p-4">
              <div className="min-w-0">
                <p className="font-medium">OpenRouter</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {provider.data?.key_set ? 'API key configured' : 'API key not configured'}
                </p>
              </div>
              {provider.data ? (
                <Badge variant="outline" className={cn(
                  provider.data.available
                    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
                    : 'border-destructive/30 bg-destructive/10 text-destructive',
                )}>
                  {provider.data.available ? <CheckCircle2 /> : <CircleAlert />}
                  {provider.data.status}
                </Badge>
              ) : <Skeleton className="h-6 w-20" />}
            </div>
            {profilesDiffer ? (
              <div className="flex gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-800 dark:text-amber-300">
                <CircleAlert className="mt-0.5 size-4 shrink-0" />
                The two Watch Tower stages currently use different models. Applying a model here will synchronize them.
              </div>
            ) : null}
            {models.isError ? (
              <p className="text-sm text-destructive">The live model catalog is unavailable. The current assignment remains unchanged.</p>
            ) : (
              <>
                <div className="relative">
                  <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="pl-9"
                    type="search"
                    placeholder="Search compatible models…"
                    value={modelSearch}
                    onChange={(event) => setModelSearch(event.target.value)}
                  />
                </div>
                <p className="text-xs text-muted-foreground">{visibleModels.length} compatible models</p>
              </>
            )}
          </div>

          <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-4" role="radiogroup" aria-label="Compatible Watch Tower models">
            {models.isPending ? [0, 1, 2, 3].map((item) => <Skeleton key={item} className="h-24 rounded-xl" />) : null}
            {visibleModels.slice(0, 75).map((model) => (
              <ModelOption
                key={model.id}
                model={model}
                selected={selectedModel === model.id}
                onSelect={setSelectedModel}
              />
            ))}
            {!models.isPending && !models.isError && visibleModels.length === 0 ? (
              <p className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">
                No compatible model matches this search.
              </p>
            ) : null}
          </div>

          <SheetFooter className="border-t sm:flex-row sm:justify-end">
            <Button variant="outline" onClick={() => setModelPickerOpen(false)}>Cancel</Button>
            <Button
              onClick={() => updateModel.mutate(selectedModel)}
              disabled={!selectedModel || updateModel.isPending || (!profilesDiffer && selectedModel === configuredModel)}
            >
              {updateModel.isPending ? <LoaderCircle className="animate-spin" /> : <Bot />}
              Apply to Watch Tower
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete conversation?</DialogTitle>
            <DialogDescription>
              This permanently removes “{deleteTarget?.title}” and its saved Watch Tower answers.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button
              variant="destructive"
              onClick={() => deleteTarget && removeThread.mutate(deleteTarget.id)}
              disabled={removeThread.isPending}
            >
              {removeThread.isPending ? <LoaderCircle className="animate-spin" /> : <Trash2 />}
              Delete conversation
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}

function ModelOption({ model, selected, onSelect }: { model: LlmModel; selected: boolean; onSelect: (model: string) => void }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={() => onSelect(model.id)}
      className={cn(
        'flex w-full items-start gap-3 rounded-xl border p-4 text-left outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring',
        selected && 'border-primary bg-primary/5 ring-1 ring-primary',
      )}
    >
      <span className={cn(
        'mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border',
        selected && 'border-primary bg-primary text-primary-foreground',
      )}>
        {selected ? <Check className="size-3" /> : null}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{model.name}</span>
        <span className="mt-1 block break-all text-xs text-muted-foreground">{model.id}</span>
        <span className="mt-3 block text-xs text-muted-foreground">
          {model.context_length.toLocaleString()} context · ${model.input_price_per_million.toLocaleString()}/1M input · ${model.output_price_per_million.toLocaleString()}/1M output
        </span>
      </span>
    </button>
  )
}

function ThreadHistoryHeader({ onNew }: { onNew: () => void }) {
  return (
    <div className="shrink-0 border-b p-3">
      <Button className="w-full justify-start" onClick={onNew}>
        <Plus /> New conversation
      </Button>
    </div>
  )
}

function ThreadHistory({
  threads,
  selectedID,
  loading,
  onSelect,
  onDelete,
}: {
  threads: WatchTowerThread[]
  selectedID: string | null
  loading: boolean
  onSelect: (id: string) => void
  onDelete: (thread: WatchTowerThread) => void
}) {
  if (loading) {
    return <div className="space-y-3 p-3">{[0, 1, 2].map((item) => <Skeleton key={item} className="h-16 rounded-xl" />)}</div>
  }
  if (!threads.length) {
    return (
      <div className="flex flex-col items-center gap-2 px-6 py-12 text-center text-muted-foreground">
        <History className="size-5" />
        <p className="text-sm">Your saved investigations will appear here.</p>
      </div>
    )
  }
  return (
    <div className="space-y-1 p-2">
      {threads.map((thread) => (
        <div
          key={thread.id}
          className={cn(
            'group flex items-start gap-1 rounded-xl px-1 py-1 transition-colors',
            selectedID === thread.id ? 'bg-accent' : 'hover:bg-accent/60',
          )}
        >
          <button type="button" className="min-w-0 flex-1 px-2 py-2 text-left" onClick={() => onSelect(thread.id)}>
            <span className="line-clamp-2 text-sm font-medium leading-5">{thread.title}</span>
            <span className="mt-1 block text-xs text-muted-foreground">
              {thread.message_count} messages · {formatRelative(thread.updated_at)}
            </span>
          </button>
          <Button
            variant="ghost"
            size="icon-sm"
            className="mt-1 shrink-0 opacity-60 sm:opacity-0 sm:group-hover:opacity-100"
            onClick={() => onDelete(thread)}
          >
            <Trash2 />
            <span className="sr-only">Delete {thread.title}</span>
          </Button>
        </div>
      ))}
    </div>
  )
}

function Welcome({ onQuestion, disabled }: { onQuestion: (question: string) => void; disabled: boolean }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center py-6 text-center sm:py-12">
      <div className="relative mb-5 flex size-16 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/20">
        <Telescope className="size-7" />
        <span className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full border-2 border-card bg-emerald-500">
          <span className="size-1.5 rounded-full bg-white" />
        </span>
      </div>
      <h2 className="font-heading text-xl font-semibold">What should we investigate?</h2>
      <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
        Watch Tower searches captured reports, compares coverage, and produces an evidence-backed briefing with links to every cited article.
      </p>
      <div className="mt-7 grid w-full max-w-2xl gap-2 sm:grid-cols-2">
        {suggestedQuestions.map((question, index) => (
          <button
            key={question}
            type="button"
            onClick={() => onQuestion(question)}
            disabled={disabled}
            className="group flex min-h-20 items-start gap-3 rounded-2xl border bg-card p-4 text-left transition-colors hover:border-primary/30 hover:bg-accent disabled:opacity-50"
          >
            <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              {index === 0 ? <Clock3 className="size-3.5" /> : index === 3 ? <BookOpenText className="size-3.5" /> : <Sparkles className="size-3.5" />}
            </span>
            <span className="text-sm font-medium leading-5">{question}</span>
          </button>
        ))}
      </div>
      <div className="mt-7 flex flex-wrap justify-center gap-2 text-xs text-muted-foreground">
        <Badge variant="secondary"><Database /> Newsroom corpus</Badge>
        <Badge variant="secondary"><BookOpenText /> Source citations</Badge>
        <Badge variant="secondary"><MessageSquareText /> Conversation memory</Badge>
      </div>
    </div>
  )
}

function ChatMessage({ message, onSuggestion, disabled }: { message: WatchTowerMessage; onSuggestion: (value: string) => void; disabled: boolean }) {
  if (message.role === 'user') {
    return (
      <div className="ml-auto max-w-[88%] rounded-2xl rounded-br-md bg-primary px-4 py-3 text-sm leading-6 text-primary-foreground sm:max-w-[75%]">
        {message.content}
      </div>
    )
  }
  return (
    <article className="flex min-w-0 gap-3 sm:gap-4">
      <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        <Telescope className="size-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <span className="text-sm font-semibold">Watch Tower</span>
          {message.search ? (
            <Badge variant="secondary" className="font-normal">
              <Clock3 /> {message.search.label} · {message.search.article_count} reports reviewed
            </Badge>
          ) : null}
        </div>
        <RichArticleContent value={message.content} className="text-[0.9375rem] leading-7" />
        {message.citations.length ? <Evidence citations={message.citations} /> : null}
        {message.suggestions.length ? (
          <div className="mt-4 flex flex-wrap gap-2">
            {message.suggestions.map((suggestion) => (
              <Button key={suggestion} variant="outline" size="sm" className="h-auto whitespace-normal py-2 text-left" onClick={() => onSuggestion(suggestion)} disabled={disabled}>
                {suggestion}
              </Button>
            ))}
          </div>
        ) : null}
        {message.model ? <p className="mt-3 text-[11px] text-muted-foreground">{message.provider} · {message.model}</p> : null}
      </div>
    </article>
  )
}

function Evidence({ citations }: { citations: WatchTowerMessage['citations'] }) {
  return (
    <div className="mt-5 rounded-2xl border bg-muted/20 p-3 sm:p-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        <BookOpenText className="size-3.5" /> Evidence used
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        {citations.map((citation) => (
          <div key={`${citation.number}-${citation.article_id}`} className="group flex min-w-0 gap-3 rounded-xl border bg-card p-3 hover:border-primary/30">
            <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-xs font-semibold text-primary">{citation.number}</span>
            <div className="min-w-0 flex-1">
              <Link to={`/articles/${citation.article_id}`} className="line-clamp-2 text-sm font-medium leading-5 hover:underline">
                {citation.headline}
              </Link>
              <p className="mt-1 truncate text-xs text-muted-foreground">{citation.source} · {dateTime.format(new Date(citation.published_at))}</p>
            </div>
            <Button variant="ghost" size="icon-sm" render={<a href={citation.original_url} target="_blank" rel="noreferrer" />}>
              <ExternalLink />
              <span className="sr-only">Open original article</span>
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}

function PendingAnswer({ question }: { question: string }) {
  return (
    <div className="mt-7 flex flex-col gap-7">
      <div className="ml-auto max-w-[88%] rounded-2xl rounded-br-md bg-primary px-4 py-3 text-sm leading-6 text-primary-foreground sm:max-w-[75%]">
        {question}
      </div>
      <div className="flex gap-3 sm:gap-4">
        <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <LoaderCircle className="size-4 animate-spin" />
        </div>
        <div className="flex-1 space-y-3 pt-1">
          <p className="text-sm font-medium">Searching the newsroom corpus…</p>
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary"><Database /> Finding relevant reports</Badge>
            <Badge variant="secondary"><BookOpenText /> Comparing sources</Badge>
          </div>
          <Skeleton className="h-3 w-full max-w-xl" />
          <Skeleton className="h-3 w-4/5 max-w-lg" />
        </div>
      </div>
    </div>
  )
}

function ConversationSkeleton() {
  return (
    <div className="space-y-8">
      <Skeleton className="ml-auto h-16 w-2/3 rounded-2xl" />
      <div className="flex gap-4">
        <Skeleton className="size-8 shrink-0 rounded-xl" />
        <div className="flex-1 space-y-3"><Skeleton className="h-4 w-28" /><Skeleton className="h-3 w-full" /><Skeleton className="h-3 w-5/6" /><Skeleton className="h-24 w-full rounded-2xl" /></div>
      </div>
    </div>
  )
}

function formatRelative(value: string) {
  const difference = new Date(value).getTime() - Date.now()
  const minutes = Math.round(difference / 60_000)
  if (Math.abs(minutes) < 60) return relativeTime.format(minutes, 'minute')
  const hours = Math.round(minutes / 60)
  if (Math.abs(hours) < 24) return relativeTime.format(hours, 'hour')
  return relativeTime.format(Math.round(hours / 24), 'day')
}
