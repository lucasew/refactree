import React, { useEffect } from "react";
import { normalizeRef, pathFromRef } from "../routes";

type Segment = {
  text: string;
  href?: string | null;
  anchorId?: string | null;
  isLink: boolean;
  isDef: boolean;
  reference?: string | null;
};

type Props = {
  segments: ReadonlyArray<Segment | null | undefined>;
  nonText: boolean;
  focusId: string | null | undefined;
  onNavigate: (ref: string) => void;
  loading?: boolean;
};

function scrollToAnchor(id: string | null | undefined) {
  if (!id) return;
  const el = document.getElementById(id);
  if (!el) return;
  el.scrollIntoView({ block: "center", behavior: "smooth" });
  el.classList.add("is-focus");
  // brief highlight for graph → source jumps
  window.setTimeout(() => el.classList.remove("is-focus"), 1600);
}

export function CodePanel({ segments, nonText, focusId, onNavigate, loading }: Props) {
  // Server focusId (preferred) or URL hash (deep link / graph click).
  useEffect(() => {
    const fromHash = window.location.hash.replace(/^#/, "");
    const id = focusId || fromHash || null;
    if (!id) return;
    // segments may paint a tick later after Relay resolve
    const t0 = requestAnimationFrame(() => scrollToAnchor(id));
    const t1 = window.setTimeout(() => scrollToAnchor(id), 50);
    const t2 = window.setTimeout(() => scrollToAnchor(id), 200);
    return () => {
      cancelAnimationFrame(t0);
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [focusId, segments]);

  if (nonText) {
    return <p className="p-4 text-sm text-base-content/50">Binary or non-text file — not rendered.</p>;
  }
  if (!segments.length) {
    return <p className="p-4 text-sm text-base-content/50">No source.</p>;
  }

  return (
    <div className="relative">
      {loading ? (
        <span className="badge badge-ghost badge-sm absolute right-2 top-2 z-10">
          refreshing…
        </span>
      ) : null}
      <pre className="code-view p-3 m-0">
        {segments.filter(Boolean).map((s, i) => {
          const key = i;
          if (s!.isLink && s!.reference) {
            const ref = normalizeRef(s!.reference);
            const focused = !!(s!.anchorId && (s!.anchorId === focusId || s!.anchorId === window.location.hash.replace(/^#/, "")));
            return (
              <a
                key={key}
                href={pathFromRef(ref)}
                id={s!.anchorId ?? undefined}
                className={[s!.isDef ? "is-def" : "", focused ? "is-focus" : ""].filter(Boolean).join(" ") || undefined}
                onClick={(e) => {
                  e.preventDefault();
                  onNavigate(ref);
                }}
              >
                {s!.text}
              </a>
            );
          }
          if (s!.isDef && s!.anchorId) {
            const focused =
              s!.anchorId === focusId || s!.anchorId === window.location.hash.replace(/^#/, "");
            return (
              <span
                key={key}
                id={s!.anchorId}
                className={focused ? "is-def is-focus" : "is-def"}
              >
                {s!.text}
              </span>
            );
          }
          return <span key={key}>{s!.text}</span>;
        })}
      </pre>
    </div>
  );
}
