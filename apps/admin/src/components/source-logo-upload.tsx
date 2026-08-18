import { ImageUp, Trash2 } from 'lucide-react'
import { useEffect, useId, useState } from 'react'
import { toast } from 'sonner'

import { SourceAvatar } from '@/components/source-avatar'
import { Button } from '@/components/ui/button'

const maxLogoBytes = 768 * 1024
const acceptedTypes = new Set(['image/jpeg', 'image/png'])

type SourceLogoUploadProps = {
  currentIconUrl?: string
  disabled?: boolean
  file: File | null
  name: string
  onFileChange: (file: File | null) => void
  onRemoveChange: (remove: boolean) => void
  remove: boolean
  website?: string
}

export function SourceLogoUpload({
  currentIconUrl,
  disabled,
  file,
  name,
  onFileChange,
  onRemoveChange,
  remove,
  website,
}: SourceLogoUploadProps) {
  const inputId = useId()
  const [preview, setPreview] = useState('')

  useEffect(() => {
    if (!file) {
      setPreview('')
      return
    }
    const objectUrl = URL.createObjectURL(file)
    setPreview(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [file])

  function selectLogo(nextFile?: File) {
    if (!nextFile) return
    if (!acceptedTypes.has(nextFile.type)) {
      toast.error('Choose a PNG or JPEG logo')
      return
    }
    if (nextFile.size > maxLogoBytes) {
      toast.error('Logo must be smaller than 768 KB')
      return
    }
    onFileChange(nextFile)
    onRemoveChange(false)
  }

  const hasStoredLogo = Boolean(currentIconUrl) && !remove

  return (
    <div className="flex items-center gap-4 rounded-xl border bg-muted/30 p-4">
      <SourceAvatar
        className="size-16 rounded-xl"
        iconUrl={remove ? '' : preview || currentIconUrl}
        name={name || 'Source'}
        website={website}
      />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">Publisher logo</p>
        <p className="mt-1 text-xs text-muted-foreground">PNG or JPEG, at least 64 × 64 px, up to 768 KB.</p>
        {file ? <p className="mt-1 truncate text-xs text-foreground">{file.name}</p> : null}
        {remove ? <p className="mt-1 text-xs text-foreground">Logo will be removed when saved.</p> : null}
        <div className="mt-3 flex flex-wrap gap-2">
          <input
            id={inputId}
            className="sr-only"
            type="file"
            accept="image/png,image/jpeg"
            disabled={disabled}
            onChange={(event) => {
              selectLogo(event.target.files?.[0])
              event.target.value = ''
            }}
          />
          <Button type="button" size="sm" variant="outline" disabled={disabled} render={<label htmlFor={inputId} />}>
            <ImageUp />
            {file || hasStoredLogo ? 'Replace logo' : 'Upload logo'}
          </Button>
          {file || hasStoredLogo ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              disabled={disabled}
              onClick={() => {
                onFileChange(null)
                onRemoveChange(hasStoredLogo)
              }}
            >
              <Trash2 />
              Remove
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  )
}
