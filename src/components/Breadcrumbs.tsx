import { Link } from '@tanstack/react-router'
import { Fragment } from 'react'
import { BreadcrumbSeparator } from './icons'
import { t } from '../i18n'

/** The path above a page title: the resource's list, then the current page. */
export function Breadcrumbs({ resource, label, items }: { resource: string; label: string; items: string[] }) {
  return (
    <nav aria-label={t('common.breadcrumb')}>
      <ol className="flex flex-wrap items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
        <li>
          <Link to="/$resource" params={{ resource }} className="rounded font-medium hover:text-gray-800 dark:hover:text-gray-100">
            {label}
          </Link>
        </li>
        {items.map((item, i) => (
          <Fragment key={i}>
            <li aria-hidden="true" className="text-gray-300 dark:text-gray-600">
              <BreadcrumbSeparator />
            </li>
            <li aria-current={i === items.length - 1 ? 'page' : undefined} className="max-w-60 truncate">
              {item}
            </li>
          </Fragment>
        ))}
      </ol>
    </nav>
  )
}
