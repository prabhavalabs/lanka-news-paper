import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@snap/ui/button'
import { Input } from '@snap/ui/input'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router'
import { z } from 'zod'

const schema = z.object({
  email: z.string().email(),
})

type Values = z.infer<typeof schema>

export function LoginPage() {
  const navigate = useNavigate()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '' },
  })

  return (
    <div className="mx-auto flex min-h-screen max-w-sm flex-col justify-center px-6">
      <h1 className="text-xl font-medium">Newsroom sign-in</h1>
      <p className="mt-2 text-sm text-[color:var(--ink-tertiary)]">
        Authentication is a later slice. This form only validates locally.
      </p>
      <form
        className="mt-8 flex flex-col gap-6"
        onSubmit={form.handleSubmit(() => navigate('/'))}
      >
        <label className="flex flex-col gap-2 text-sm">
          Email
          <Input type="email" autoComplete="username" {...form.register('email')} />
        </label>
        {form.formState.errors.email ? (
          <p className="text-sm text-[color:var(--error)]">{form.formState.errors.email.message}</p>
        ) : null}
        <Button type="submit">Continue</Button>
      </form>
    </div>
  )
}
