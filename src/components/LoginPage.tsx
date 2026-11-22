import { useState, type FormEvent } from 'react'
import { ApiError, type ApiClient } from '../api'
import type { FieldErrors } from '../protocol'
import { inputClass } from './fields'
import { t } from '../i18n'

export function LoginPage({ api, brand, onLoggedIn }: { api: ApiClient; brand: string; onLoggedIn: () => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [general, setGeneral] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setPending(true)
    setErrors({})
    setGeneral(null)
    try {
      await api.login(email, password)
      onLoggedIn()
    } catch (err) {
      if (err instanceof ApiError && err.isValidation) setErrors(err.errors)
      else if (err instanceof ApiError && err.status === 429) setGeneral(t('login.tooMany'))
      else setGeneral(err instanceof Error ? err.message : String(err))
    } finally {
      setPending(false)
    }
  }

  const fieldClass = (invalid: boolean) =>
    inputClass + ' py-2.5 ' + (invalid ? 'ring-danger-500 focus:ring-danger-500 dark:ring-danger-400' : '')

  return (
    <div className="grid min-h-screen text-gray-950 lg:grid-cols-2 dark:text-white">
      <div className="flex items-center justify-center px-6 py-12">
        <form onSubmit={submit} noValidate className="w-full max-w-sm animate-page space-y-7">
          <div className="space-y-4">
            <span
              aria-hidden="true"
              className="flex size-12 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-400 to-primary-700 text-xl font-bold text-white shadow-lg shadow-primary-600/30 ring-1 ring-inset ring-white/20"
            >
              {brand.trim().charAt(0)}
            </span>
            <div>
              <h1 className="text-2xl font-bold tracking-tight">{t('login.welcome')}</h1>
              <p className="mt-1.5 text-sm text-gray-500 dark:text-gray-400">{t('login.intro', { brand })}</p>
            </div>
          </div>

          {general && (
            <p role="alert" className="rounded-xl bg-danger-50 p-3 text-sm text-danger-700 ring-1 ring-danger-600/10 dark:bg-danger-500/10 dark:text-danger-300">
              {general}
            </p>
          )}

          <div className="space-y-5">
            <div className="space-y-2">
              <label htmlFor="login-email" className="block text-sm font-medium">
                {t('login.email')}
              </label>
              <input
                id="login-email"
                type="email"
                dir="ltr"
                autoComplete="username"
                autoFocus
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                aria-invalid={!!errors.email || undefined}
                aria-describedby={errors.email ? 'login-email-error' : undefined}
                className={fieldClass(!!errors.email)}
              />
              {errors.email && (
                <p id="login-email-error" className="text-sm text-danger-600 dark:text-danger-400">
                  {errors.email.join(' ')}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <label htmlFor="login-password" className="block text-sm font-medium">
                {t('login.password')}
              </label>
              <input
                id="login-password"
                type="password"
                dir="ltr"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={!!errors.password || undefined}
                aria-describedby={errors.password ? 'login-password-error' : undefined}
                className={fieldClass(!!errors.password)}
              />
              {errors.password && (
                <p id="login-password-error" className="text-sm text-danger-600 dark:text-danger-400">
                  {errors.password.join(' ')}
                </p>
              )}
            </div>
          </div>

          <button
            type="submit"
            disabled={pending}
            className="flex w-full items-center justify-center gap-2 rounded-xl bg-primary-600 px-4 py-2.5 text-sm font-semibold text-white shadow-md shadow-primary-600/30 ring-1 ring-inset ring-white/10 hover:bg-primary-500 active:scale-[0.99] disabled:opacity-60"
          >
            {pending && <span className="size-4 animate-spin rounded-full border-2 border-white/30 border-t-white" aria-hidden="true" />}
            {pending ? t('login.submitting') : t('login.submit')}
          </button>
        </form>
      </div>

      {/* Decorative side: only on wide screens. */}
      <div aria-hidden="true" className="relative hidden overflow-hidden bg-primary-950 lg:block">
        <div className="absolute inset-0 bg-[radial-gradient(40rem_30rem_at_20%_10%,var(--color-primary-500),transparent),radial-gradient(36rem_28rem_at_90%_90%,var(--color-info-500),transparent)] opacity-70" />
        <div className="absolute inset-0 bg-[linear-gradient(to_right,rgb(255_255_255/0.06)_1px,transparent_1px),linear-gradient(to_bottom,rgb(255_255_255/0.06)_1px,transparent_1px)] bg-[size:48px_48px] [mask-image:radial-gradient(ellipse_at_center,black,transparent_75%)]" />
        <div className="relative flex h-full flex-col justify-end p-12 text-white">
          <p className="text-3xl leading-snug font-bold">{t('login.heroTitle')}</p>
          <p className="mt-3 max-w-md text-base leading-7 text-white/70">{t('login.heroText')}</p>
        </div>
      </div>
    </div>
  )
}
