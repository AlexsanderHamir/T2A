import type { ReactNode } from "react";

type SectionVariant = "essentials" | "context" | "execution";

type Props = {
  title: string;
  lede?: string;
  children: ReactNode;
  variant: SectionVariant;
};

export function TaskCreateModalSection({
  title,
  lede,
  children,
  variant,
}: Props) {
  const headId = `task-create-modal-section-${variant}`;

  return (
    <section
      className={`task-create-modal-panel task-create-modal-panel--${variant}`}
      aria-labelledby={headId}
    >
      <header className="task-create-modal-panel__head">
        <h3 id={headId} className="task-create-modal-panel__title">
          {title}
        </h3>
        {lede ? (
          <p className="task-create-modal-panel__lede">{lede}</p>
        ) : null}
      </header>
      <div className="task-create-modal-panel__body">{children}</div>
    </section>
  );
}
