import { createClient } from '@snap/api-client'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

const client = createClient()
const schema = z.object({
  email: z.string().email('Enter a valid email address.'),
  password: z.string().min(8, 'Enter your password.').max(72, 'Password is too long.'),
})
type Values = z.infer<typeof schema>

export function LoginPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })

  return (
    <main className="relative flex min-h-svh items-center justify-center bg-background px-6 py-16 text-foreground">
      <section className="w-full max-w-sm rounded-2xl border border-border bg-card p-6 shadow-sm" aria-labelledby="login-title">
        <h1 id="login-title" className="font-heading text-lg font-semibold tracking-tight">Login to your account</h1>
        <p className="mt-2 text-sm leading-5 text-muted-foreground">
          Sign in with the predefined administrator account.
        </p>
        <form
          className="mt-6"
          onSubmit={form.handleSubmit(async (values) => {
            form.clearErrors('root')
            try {
              const result = await client.login(values.email, values.password)
              queryClient.setQueryData(['me'], result.user)
              navigate('/', { replace: true })
            } catch {
              form.setError('root', { message: 'The email or password is incorrect.' })
            }
          })}
        >
          <FieldGroup className="gap-6">
            <Field data-invalid={Boolean(form.formState.errors.email)}>
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                autoComplete="username"
                placeholder="admin@lankanews.com"
                className="h-9 rounded-lg border border-border bg-muted/40 px-3"
                aria-invalid={Boolean(form.formState.errors.email)}
                {...form.register('email')}
              />
              <FieldError errors={[form.formState.errors.email]} />
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.password)}>
              <FieldLabel htmlFor="password">Password</FieldLabel>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                maxLength={72}
                className="h-9 rounded-lg border border-border bg-muted/40 px-3"
                aria-invalid={Boolean(form.formState.errors.password)}
                {...form.register('password')}
              />
              <FieldError errors={[form.formState.errors.password]} />
            </Field>
            <FieldError errors={[form.formState.errors.root]} />
            <Button type="submit" size="lg" className="h-9 w-full rounded-lg" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? 'Signing in…' : 'Login'}
            </Button>
          </FieldGroup>
        </form>
      </section>
    </main>
  )
}
