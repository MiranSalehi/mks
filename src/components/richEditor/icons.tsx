import { useId, type ReactNode } from 'react'

// The toolbar's icons: 20px line icons in the look of CKEditor 5's (drawn for this panel).

function Svg({ children, fill }: { children: ReactNode; fill?: boolean }) {
  return (
    <svg
      viewBox="0 0 20 20"
      width="20"
      height="20"
      fill={fill ? 'currentColor' : 'none'}
      stroke={fill ? 'none' : 'currentColor'}
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className="shrink-0"
    >
      {children}
    </svg>
  )
}

const Letter = ({ x = 10, y = 15, size = 14, weight = 400, italic, children }: { x?: number; y?: number; size?: number; weight?: number; italic?: boolean; children: string }) => (
  <text
    x={x}
    y={y}
    textAnchor="middle"
    fontSize={size}
    fontWeight={weight}
    fontStyle={italic ? 'italic' : undefined}
    fontFamily="Helvetica, Arial, sans-serif"
    fill="currentColor"
    stroke="none"
  >
    {children}
  </text>
)

/** A letter cut out of a filled square (its own mask id: the toolbar is drawn twice). */
function BackgroundColorIcon() {
  const id = useId()
  return (
    <Svg>
      <mask id={id}>
        <rect width="20" height="20" fill="white" />
        <text x="10" y="14.5" textAnchor="middle" fontSize="12" fontFamily="Helvetica, Arial, sans-serif" fill="black">
          A
        </text>
      </mask>
      <rect x="3" y="3" width="14" height="14" rx="2" fill="currentColor" stroke="none" mask={`url(#${id})`} />
    </Svg>
  )
}

export const icons = {
  undo: (
    <Svg>
      <path d="M7.5 4.5 4 8l3.5 3.5M4 8h8.5a4 4 0 0 1 0 8H9" />
    </Svg>
  ),
  redo: (
    <Svg>
      <path d="M12.5 4.5 16 8l-3.5 3.5M16 8H7.5a4 4 0 0 0 0 8H11" />
    </Svg>
  ),
  source: (
    <Svg>
      <path d="M11 2.5H5.25a1 1 0 0 0-1 1v13a1 1 0 0 0 1 1H9M11 2.5l4.25 4.25V9.5M11 2.5v4.25h4.25" />
      <path d="m12.75 12.25-2 2 2 2M15.75 12.25l2 2-2 2" />
    </Svg>
  ),
  showBlocks: (
    <Svg>
      <rect x="2.5" y="3" width="15" height="14" rx="0.5" strokeDasharray="1.4 1.6" strokeWidth={1.2} />
      <Letter y={13.25} size={7.5} weight={700}>
        H1
      </Letter>
    </Svg>
  ),
  fontSize: (
    <Svg>
      <Letter x={8} y={16} size={15}>
        A
      </Letter>
      <path d="M16.25 4v12M14.5 5.75 16.25 4 18 5.75M14.5 14.25 16.25 16 18 14.25" strokeWidth={1.2} />
    </Svg>
  ),
  fontFamily: (
    <Svg>
      <Letter x={7.5} y={16} size={15}>
        A
      </Letter>
      <path d="M13.5 5h5M14.5 8.5h4M15.5 12h3" strokeWidth={1.2} />
    </Svg>
  ),
  fontColor: (
    <Svg>
      <Letter y={13.5} size={13}>
        A
      </Letter>
      <path d="M4 17h12" strokeWidth={2} />
    </Svg>
  ),
  fontBackgroundColor: <BackgroundColorIcon />,
  bold: (
    <Svg>
      <Letter y={15.5} size={15} weight={700}>
        B
      </Letter>
    </Svg>
  ),
  italic: (
    <Svg>
      <Letter y={15.5} size={15} italic>
        I
      </Letter>
    </Svg>
  ),
  underline: (
    <Svg>
      <Letter y={14} size={14}>
        U
      </Letter>
      <path d="M5 17.25h10" strokeWidth={1.3} />
    </Svg>
  ),
  link: (
    <Svg>
      <path d="M8.5 11.5a3 3 0 0 0 4.24 0l2.5-2.5A3 3 0 0 0 11 4.76l-1 1" />
      <path d="M11.5 8.5a3 3 0 0 0-4.24 0l-2.5 2.5A3 3 0 0 0 9 15.24l1-1" />
    </Svg>
  ),
  table: (
    <Svg>
      <rect x="2.75" y="3.5" width="14.5" height="13" rx="1" />
      <path d="M2.75 7.75h14.5M2.75 12.25h14.5M7.6 3.5v13M12.4 3.5v13" strokeWidth={1.2} />
    </Svg>
  ),
  highlight: (
    <Svg>
      <path d="m11.5 3.25 4.25 4.25-6.5 6.5H5v-4.25z" />
      <path d="m9.5 5.25 4.25 4.25" strokeWidth={1.2} />
      <path d="M3 17.5h14" strokeWidth={2.5} stroke="var(--rte-marker, currentColor)" />
    </Svg>
  ),
  blockQuote: (
    <Svg fill>
      <path d="M3 10.5c0-3.1 1.6-5.2 4.6-6.2l.6 1.1C6.6 6.2 5.8 7.4 5.7 9H8.5v5.25H3zm8.5 0c0-3.1 1.6-5.2 4.6-6.2l.6 1.1c-1.6.8-2.4 2-2.5 3.6H17v5.25h-5.5z" />
    </Svg>
  ),
  codeBlock: (
    <Svg>
      <rect x="2.75" y="3" width="14.5" height="14" rx="1" />
      <path d="M8 7.5 5.75 10 8 12.5M12 7.5l2.25 2.5L12 12.5" />
    </Svg>
  ),
  alignLeft: (
    <Svg>
      <path d="M3 4.5h14M3 8.5h9M3 12.5h14M3 16.5h9" />
    </Svg>
  ),
  alignRight: (
    <Svg>
      <path d="M3 4.5h14M8 8.5h9M3 12.5h14M8 16.5h9" />
    </Svg>
  ),
  alignCenter: (
    <Svg>
      <path d="M3 4.5h14M5.5 8.5h9M3 12.5h14M5.5 16.5h9" />
    </Svg>
  ),
  alignJustify: (
    <Svg>
      <path d="M3 4.5h14M3 8.5h14M3 12.5h14M3 16.5h14" />
    </Svg>
  ),
  bulletedList: (
    <Svg>
      <path d="M8 5h9M8 10h9M8 15h9" />
      <circle cx="4" cy="5" r="1.25" fill="currentColor" stroke="none" />
      <circle cx="4" cy="10" r="1.25" fill="currentColor" stroke="none" />
      <circle cx="4" cy="15" r="1.25" fill="currentColor" stroke="none" />
    </Svg>
  ),
  numberedList: (
    <Svg>
      <path d="M8.5 5h8.5M8.5 10h8.5M8.5 15h8.5" />
      <Letter x={4} y={7.5} size={6.5} weight={700}>
        1
      </Letter>
      <Letter x={4} y={17.5} size={6.5} weight={700}>
        2
      </Letter>
    </Svg>
  ),
  todoList: (
    <Svg>
      <path d="m2.75 5 1.25 1.25L6.5 3.75M2.75 10.5l1.25 1.25 2.5-2.5" strokeWidth={1.3} />
      <path d="M9 5h8M9 10.5h8M9 16h8" />
      <rect x="3" y="14.25" width="3.5" height="3.5" rx="0.5" strokeWidth={1.1} />
    </Svg>
  ),
  indent: (
    <Svg>
      <path d="M3 4h14M9.5 8h7.5M9.5 12h7.5M3 16h14" />
      <path d="M3.25 7.5 6.25 10l-3 2.5" />
    </Svg>
  ),
  outdent: (
    <Svg>
      <path d="M3 4h14M9.5 8h7.5M9.5 12h7.5M3 16h14" />
      <path d="M6.25 7.5 3.25 10l3 2.5" />
    </Svg>
  ),
  more: (
    <Svg fill>
      <circle cx="10" cy="4.5" r="1.6" />
      <circle cx="10" cy="10" r="1.6" />
      <circle cx="10" cy="15.5" r="1.6" />
    </Svg>
  ),
  check: (
    <Svg>
      <path d="m4.5 10.5 3.5 3.5 7.5-8" strokeWidth={1.8} />
    </Svg>
  ),
  cancel: (
    <Svg>
      <path d="m5 5 10 10M15 5 5 15" strokeWidth={1.8} />
    </Svg>
  ),
  unlink: (
    <Svg>
      <path d="M8.5 11.5a3 3 0 0 0 4.24 0l2.5-2.5A3 3 0 0 0 11 4.76l-1 1" />
      <path d="M11.5 8.5a3 3 0 0 0-4.24 0l-2.5 2.5A3 3 0 0 0 9 15.24l1-1M3 3l14 14" />
    </Svg>
  ),
  eraser: (
    <Svg>
      <path d="m8 16.5-4.5-4.5L11 4.5l5.5 5.5-6.5 6.5zM6.5 9l5.5 5.5M8 16.5h9" />
    </Svg>
  ),
}

/** The small arrow of a dropdown button. */
export const Arrow = () => (
  <svg viewBox="0 0 10 10" width="10" height="10" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="shrink-0 opacity-80">
    <path d="m2 3.75 3 3 3-3" />
  </svg>
)
