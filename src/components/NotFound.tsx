import { Link } from '@tanstack/react-router'
import { formatNumber, t } from '../i18n'

export function NotFound() {
  return (
    <div className="py-24 text-center">
      <p className="bg-gradient-to-b from-primary-400 to-primary-700 bg-clip-text text-7xl font-black text-transparent">{formatNumber(404, { useGrouping: false })}</p>
      <p className="mt-4 text-gray-600 dark:text-gray-400">{t('error.notFound')}</p>
      <Link to="/" className="mt-8 inline-flex items-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 hover:bg-primary-500">
        {t('common.home')}
      </Link>
    </div>
  )
}

export function Forbidden() {
  return (
    <div className="py-24 text-center">
      <p className="bg-gradient-to-b from-danger-400 to-danger-700 bg-clip-text text-7xl font-black text-transparent">{formatNumber(403, { useGrouping: false })}</p>
      <p className="mt-4 text-gray-600 dark:text-gray-400">{t('error.forbidden')}</p>
      <Link to="/" className="mt-8 inline-flex items-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 hover:bg-primary-500">
        {t('common.home')}
      </Link>
    </div>
  )
}
