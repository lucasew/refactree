import React, { useEffect } from "react";

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
};

export function CodePanel({ segments, nonText, focusId, onNavigate }: Props) {
  useEffect(() => {
    if (!focusId) return;
    const el = document.getElementById(focusId);
    el?.scrollIntoView({ block: "center" });
  }, [focusId, segments]);

  if (nonText) {
    return <p className="muted">Binary or non-text file — not rendered.</p>;
  }
  if (!segments.length) {
    return <p className="muted">No source.</p>;
  }

  return (
    <pre className="code-pre">
      {segments.filter(Boolean).map((s, i) => {
        const key = i;
        if (s!.isLink && s!.reference) {
          return (
            <a
              key={key}
              href={"/code/" + encodeURIComponent(s!.reference)}
              id={s!.anchorId ?? undefined}
              className={s!.isDef ? "def" : undefined}
              onClick={(e) => {
                e.preventDefault();
                onNavigate(s!.reference!);
              }}
            >
              {s!.text}
            </a>
          );
        }
        if (s!.isDef && s!.anchorId) {
          return (
            <span key={key} id={s!.anchorId} className="def">
              {s!.text}
            </span>
          );
        }
        return <span key={key}>{s!.text}</span>;
      })}
    </pre>
  );
}
