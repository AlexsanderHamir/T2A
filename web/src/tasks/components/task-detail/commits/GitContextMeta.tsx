import { buildGitContextItems, type GitContextFields } from "./commitDisplay";

type Props = {
  context: GitContextFields;
};

export function GitContextMeta({ context }: Props) {
  const items = buildGitContextItems(context);
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="task-commits-context" data-testid="task-commits-context">
      <dl className="task-commits-context-list">
        {items.map((item) => (
          <div key={item.label} className="task-commits-context-item">
            <dt className="task-commits-context-label">{item.label}</dt>
            <dd className="task-commits-context-value" title={item.title}>
              {item.value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
