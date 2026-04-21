import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { getCycleFailures } from "@/api/tasks";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { taskQueryKeys } from "@/tasks/task-query";
import { CycleFailuresTable } from "./CycleFailuresTable";

const PAGE_SIZE = 50;

const SORT_VALUES = [
  "at_desc",
  "at_asc",
  "reason_asc",
  "reason_desc",
] as const;

type CycleFailureSort = (typeof SORT_VALUES)[number];

function isCycleFailureSort(s: string): s is CycleFailureSort {
  return (SORT_VALUES as readonly string[]).includes(s);
}

const SORT_LABELS: Record<CycleFailureSort, string> = {
  at_desc: "Time (newest first)",
  at_asc: "Time (oldest first)",
  reason_asc: "Reason (A–Z)",
  reason_desc: "Reason (Z–A)",
};

function parseOffset(searchParams: URLSearchParams): number {
  const raw = searchParams.get("offset");
  if (raw === null || raw === "") return 0;
  const n = Number.parseInt(raw, 10);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) return 0;
  return n;
}

function parseSort(searchParams: URLSearchParams): CycleFailureSort {
  const raw = searchParams.get("sort");
  if (raw === null || raw === "") return "at_desc";
  return isCycleFailureSort(raw) ? raw : "at_desc";
}

function failuresListSearch(sort: CycleFailureSort, offset: number): string {
  const q = new URLSearchParams();
  if (sort !== "at_desc") q.set("sort", sort);
  if (offset > 0) q.set("offset", String(offset));
  const s = q.toString();
  return s === "" ? "" : `?${s}`;
}

/**
 * Full list of cycle failures with server-side pagination and sort.
 * URL: `/observability/failures?sort=at_desc&offset=0`
 */
export function ObservabilityFailuresPage() {
  useDocumentTitle("Cycle failures");
  const [searchParams, setSearchParams] = useSearchParams();
  const sort = useMemo(() => parseSort(searchParams), [searchParams]);
  const offset = useMemo(() => parseOffset(searchParams), [searchParams]);

  const query = useQuery({
    queryKey: taskQueryKeys.cycleFailures(sort, offset),
    queryFn: async ({ signal }) =>
      getCycleFailures({
        signal,
        limit: PAGE_SIZE,
        offset,
        sort,
      }),
  });

  const data = query.data;
  const total = data?.total ?? 0;
  const canPrev = offset > 0;
  const canNext = data !== undefined && offset + data.failures.length < total;

  return (
    <div className="obs-page obs-failures-page task-detail-content--enter">
      <nav className="obs-failures-back" aria-label="Breadcrumb">
        <Link to="/observability" className="obs-failures-back-link">
          ← Observability
        </Link>
      </nav>
      <header className="obs-page-header">
        <h2 className="obs-page-title">Cycle failures</h2>
        <p className="obs-page-subtitle">
          All recorded cycle_failed outcomes, with deep links to audit events.
          Default order is newest first; reason-based sorts may truncate very
          large histories (see notice below when applicable).
        </p>
      </header>

      <section className="obs-failures obs-failures--full" aria-label="All cycle failures">
        <div className="obs-failures-toolbar">
          <label className="obs-failures-sort-label" htmlFor="obs-failures-sort">
            Sort by
          </label>
          <select
            id="obs-failures-sort"
            className="obs-failures-sort"
            value={sort}
            onChange={(e) => {
              const v = e.target.value;
              if (!isCycleFailureSort(v)) return;
              setSearchParams((prev) => {
                const next = new URLSearchParams(prev);
                if (v === "at_desc") next.delete("sort");
                else next.set("sort", v);
                next.delete("offset");
                return next;
              });
            }}
          >
            {SORT_VALUES.map((s) => (
              <option key={s} value={s}>
                {SORT_LABELS[s]}
              </option>
            ))}
          </select>
          <p className="obs-failures-toolbar-meta" aria-live="polite">
            {query.isPending
              ? "Loading…"
              : `${total} failure${total === 1 ? "" : "s"} total`}
          </p>
        </div>

        {data?.reason_sort_truncated ? (
          <p className="obs-failures-truncate-note" role="status">
            Reason sort uses a bounded sample of the newest failures when the
            database has more than a few thousand rows — counts and pagination
            still reflect the filtered slice.
          </p>
        ) : null}

        {query.isError ? (
          <p className="obs-failures-error" role="alert">
            Could not load failures.{" "}
            <button
              type="button"
              className="obs-failures-retry"
              onClick={() => void query.refetch()}
            >
              Retry
            </button>
          </p>
        ) : null}

        {query.isPending && !data ? (
          <p className="obs-failures-caption">Loading failures…</p>
        ) : null}

        {!query.isPending && !query.isError && data && data.failures.length === 0 ? (
          <p className="obs-failures-caption">No cycle failures recorded.</p>
        ) : null}

        {data && data.failures.length > 0 ? (
          <div
            className="obs-failures-tablewrap"
            role="region"
            aria-label="Cycle failures table"
          >
            <CycleFailuresTable failures={data.failures} />
          </div>
        ) : null}

        {data && data.failures.length > 0 ? (
          <nav className="obs-failures-pager" aria-label="Pagination">
            {canPrev ? (
              <Link
                className="obs-failures-pager-link"
                to={{
                  pathname: "/observability/failures",
                  search: failuresListSearch(sort, Math.max(0, offset - PAGE_SIZE)),
                }}
              >
                Previous
              </Link>
            ) : (
              <span className="obs-failures-pager-muted">Previous</span>
            )}
            <span className="obs-failures-pager-pos">
              {offset + 1}–{offset + data.failures.length} of {total}
            </span>
            {canNext ? (
              <Link
                className="obs-failures-pager-link"
                to={{
                  pathname: "/observability/failures",
                  search: failuresListSearch(sort, offset + PAGE_SIZE),
                }}
              >
                Next
              </Link>
            ) : (
              <span className="obs-failures-pager-muted">Next</span>
            )}
          </nav>
        ) : null}
      </section>
    </div>
  );
}
