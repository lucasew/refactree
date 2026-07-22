import React, { useState } from "react";
import { normalizeRef, pathFromRef } from "../routes";

type Entry = {
  name: string;
  reference: string;
  isDir: boolean;
};

type Props = {
  files: ReadonlyArray<Entry | null | undefined>;
  atoms: ReadonlyArray<Entry | null | undefined>;
  activeRef: string;
  onSelect: (ref: string) => void;
  atomsLoading?: boolean;
};

export function FileRail({
  files,
  atoms,
  activeRef,
  onSelect,
  atomsLoading,
}: Props) {
  const [tab, setTab] = useState<"files" | "atoms">("files");

  return (
    <div className="p-2 flex flex-col gap-2">
      <div role="tablist" className="tabs tabs-box tabs-sm w-full">
        <button
          type="button"
          role="tab"
          className={`tab flex-1 ${tab === "files" ? "tab-active" : ""}`}
          onClick={() => setTab("files")}
        >
          Files
        </button>
        <button
          type="button"
          role="tab"
          className={`tab flex-1 ${tab === "atoms" ? "tab-active" : ""}`}
          onClick={() => setTab("atoms")}
        >
          Atoms
          {atomsLoading ? (
            <span className="loading loading-spinner loading-xs ml-1" />
          ) : null}
        </button>
      </div>

      {tab === "files" ? (
        <ul className="menu menu-sm bg-base-200 rounded-box w-full p-0">
          {files.filter(Boolean).map((f) => {
            const ref = normalizeRef(f!.reference);
            return (
            <li key={ref + f!.name}>
              <a
                href={pathFromRef(ref)}
                className={ref === normalizeRef(activeRef) ? "menu-active" : ""}
                onClick={(e) => {
                  e.preventDefault();
                  onSelect(ref);
                }}
              >
                {f!.isDir ? (
                  <span className="badge badge-ghost badge-xs">dir</span>
                ) : null}
                <span className="truncate">{f!.name}</span>
              </a>
            </li>
            );
          })}
          {!files.length ? (
            <li className="menu-disabled">
              <span className="text-base-content/50">—</span>
            </li>
          ) : null}
        </ul>
      ) : (
        <ul className="menu menu-sm bg-base-200 rounded-box w-full p-0">
          {atoms.filter(Boolean).map((s) => {
            const ref = normalizeRef(s!.reference);
            return (
            <li key={ref + s!.name}>
              <button
                type="button"
                className={ref === normalizeRef(activeRef) ? "menu-active" : ""}
                onClick={() => onSelect(ref)}
              >
                <span className="truncate">{s!.name}</span>
              </button>
            </li>
            );
          })}
          {!atoms.length ? (
            <li className="menu-disabled">
              <span className="text-base-content/50">
                {atomsLoading ? "Loading atoms…" : "Open a file to list atoms."}
              </span>
            </li>
          ) : null}
        </ul>
      )}
    </div>
  );
}
