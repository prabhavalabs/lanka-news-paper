import { createClient } from '@snap/api-client'
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

const client = createClient()
const schema = z.object({ email: z.string().email(), password: z.string().min(8) })
type Values = z.infer<typeof schema>

export function LoginPage() {
  const navigate = useNavigate()
  const [mfa, setMfa] = useState<{ token: string; url?: string; qr?: string } | null>(null)
  const [code, setCode] = useState('')
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: 'editor@local.test', password: 'changeme-local' },
  })

  return (
    <div className="mx-auto flex min-h-screen max-w-sm flex-col justify-center px-6">
      <h1 className="text-xl font-medium">Newsroom sign-in</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        Local login is prefilled. First sign-in shows a TOTP QR; scan it, then enter the code.
      </p>
      {mfa ? (
        <form
          className="mt-8 flex flex-col gap-4"
          onSubmit={async (event) => {
            event.preventDefault()
            try {
              await client.verifyMfa(mfa.token, code)
              navigate('/')
            } catch {
              toast.error('Invalid code')
            }
          }}
        >
          {mfa.qr ? (
            <img src={mfa.qr} alt="Authenticator QR code" className="mx-auto size-48 border border-border bg-white p-2" />
          ) : null}
          {mfa.url ? <p className="break-all text-xs text-muted-foreground">{mfa.url}</p> : null}
          <Field>
            <FieldLabel htmlFor="code">Authenticator code</FieldLabel>
            <Input id="code" value={code} onChange={(event) => setCode(event.target.value)} autoComplete="one-time-code" />
          </Field>
          <Button type="submit">Verify</Button>
        </form>
      ) : (
        <form
          className="mt-8"
          onSubmit={form.handleSubmit(async (values) => {
            try {
              const result = await client.login(values.email, values.password)
              setMfa({ token: result.mfa_token, url: result.otpauth_url, qr: result.otpauth_qr })
            } catch {
              toast.error('Invalid credentials')
            }
          })}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input id="email" type="email" autoComplete="username" {...form.register('email')} />
            </Field>
            <Field>
              <FieldLabel htmlFor="password">Password</FieldLabel>
              <Input id="password" type="password" autoComplete="current-password" {...form.register('password')} />
            </Field>
            <Button type="submit">Continue</Button>
          </FieldGroup>
        </form>
      )}
    </div>
  )
}
