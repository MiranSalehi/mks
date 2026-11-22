import { createContext } from 'react'

/** The resource whose form is rendered; relation selects load options through it. */
export const FormResourceContext = createContext<string>('')

/** The record being edited (none when creating); uploads name it so the server can check Update. */
export const RecordKeyContext = createContext<string | undefined>(undefined)
