type IconProps = {
  className?: string;
};

export function WorktreesPlusIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth={2} strokeLinecap="round" />
    </svg>
  );
}

export function WorktreesChevronDownIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <path
        d="m6 9 6 6 6-6"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function WorktreesRefreshIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <path
        d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M3 3v5h5M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M16 16h5v5"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function WorktreesFolderIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <path
        d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function WorktreesBranchIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <circle cx={6} cy={6} r={2.5} stroke="currentColor" strokeWidth={1.75} />
      <circle cx={18} cy={18} r={2.5} stroke="currentColor" strokeWidth={1.75} />
      <path
        d="M6 8.5v3.5a3 3 0 0 0 3 3h6a3 3 0 0 0 3-3V8.5"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
      />
    </svg>
  );
}

export function WorktreesMoreIcon({ className }: IconProps) {
  return (
    <svg
      className={className}
      width={16}
      height={16}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <circle cx={12} cy={5} r={1.5} fill="currentColor" />
      <circle cx={12} cy={12} r={1.5} fill="currentColor" />
      <circle cx={12} cy={19} r={1.5} fill="currentColor" />
    </svg>
  );
}
