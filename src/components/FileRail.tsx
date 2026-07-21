import React, { useState } from "react";

type Entry = {
  name: string;
  reference: string;
  isDir: boolean;
};

type Props = {
  files: ReadonlyArray<Entry | null | undefined>;
  symbols: ReadonlyArray<Entry | null | undefined>;
  activeRef: string;
  onSelect: (ref: string) => void;
};

export function FileRail({ files, symbols, activeRef, onSelect }: Props) {
  const [tab, setTab] = useState<"files" | "symbols">("files");

  return (
    <div className="rail-tabs">
      <div className="rail-tab-labels" role="tablist">
        <button
          type="button"
          role="tab"
          className={tab === "files" ? "is-active" : ""}
          onClick={() => setTab("files")}
        >
          Files
        </button>
        <button
          type="button"
          role="tab"
          className={tab === "symbols" ? "is-active" : ""}
          onClick={() => setTab("symbols")}
        >
          Symbols
        </button>
      </div>
      {tab === "files" ? (
        <ul className="fs-list">
          {files.filter(Boolean).map((f) => (
            <li key={f!.reference + f!.name}>
              <a
                href={"/code/" + encodeURIComponent(f!.reference)}
                className={f!.reference === activeRef ? "is-active" : ""}
                onClick={(e) => {
                  e.preventDefault();
                  onSelect(f!.reference);
                }}
              >
                {f!.name}
              </a>
            </li>
          ))}
          {!files.length ? <li className="muted">—</li> : null}
        </ul>
      ) : (
        <ul className="sym-list">
          {symbols.filter(Boolean).map((s) => (
            <li key={s!.reference + s!.name}>
              <button
                type="button"
                className={s!.reference === activeRef ? "is-active" : ""}
                onClick={() => onSelect(s!.reference)}
              >
                {s!.name}
              </button>
            </li>
          ))}
          {!symbols.length ? <li className="muted">Open a file to list symbols.</li> : null}
        </ul>
      )}
    </div>
  );
}
