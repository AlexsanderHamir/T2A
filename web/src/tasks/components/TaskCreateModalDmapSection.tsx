import { FieldLabel } from "@/shared/FieldLabel";

type Props = {
  dmapCommitLimit: string;
  dmapDomain: string;
  dmapDescription: string;
  onDmapCommitLimitChange: (value: string) => void;
  onDmapDomainChange: (value: string) => void;
  onDmapDescriptionChange: (value: string) => void;
  disabled: boolean;
};

export function TaskCreateModalDmapSection({
  dmapCommitLimit,
  dmapDomain,
  dmapDescription,
  onDmapCommitLimitChange,
  onDmapDomainChange,
  onDmapDescriptionChange,
  disabled,
}: Props) {
  return (
    <section className="task-create-dmap" aria-label="DMAP task configuration">
      <h3 className="task-create-dmap-title">DMAP configuration</h3>
      <div className="row">
        <div className="field grow">
          <FieldLabel htmlFor="task-new-dmap-commit-limit" requirement="required">
            Commits until stoppage
          </FieldLabel>
          <input
            id="task-new-dmap-commit-limit"
            type="number"
            min={1}
            step={1}
            inputMode="numeric"
            value={dmapCommitLimit}
            onChange={(ev) => onDmapCommitLimitChange(ev.target.value)}
            placeholder="e.g. 8"
            required
            aria-required="true"
            disabled={disabled}
          />
        </div>
        <div className="field grow">
          <FieldLabel htmlFor="task-new-dmap-domain" requirement="required">
            DMAP domain
          </FieldLabel>
          <select
            id="task-new-dmap-domain"
            value={dmapDomain}
            onChange={(ev) => onDmapDomainChange(ev.target.value)}
            required
            aria-required="true"
            disabled={disabled}
          >
            <option value="">Choose domain</option>
            <option value="frontend">Frontend</option>
            <option value="backend">Backend</option>
            <option value="fullstack">Fullstack</option>
            <option value="devops">DevOps</option>
            <option value="data">Data</option>
            <option value="qa">QA</option>
          </select>
        </div>
      </div>
      <div className="field grow">
        <FieldLabel htmlFor="task-new-dmap-description" requirement="optional">
          Direction notes
        </FieldLabel>
        <textarea
          id="task-new-dmap-description"
          value={dmapDescription}
          onChange={(ev) => onDmapDescriptionChange(ev.target.value)}
          placeholder="Optional guidance for this DMAP run."
          rows={4}
          disabled={disabled}
        />
      </div>
    </section>
  );
}
