import { useEffect, useState } from "react";
import {
  fetchQuery,
  type GraphQLTaggedNode,
  type OperationType,
} from "relay-runtime";
import { environment } from "./RelayEnvironment";

export type QueryState<T> = {
  data: T | null;
  error: Error | null;
  loading: boolean;
};

/**
 * Non-Suspense Relay fetch: shell stays painted; panels update as results land.
 */
export function useRelayQuery<TOperation extends OperationType>(
  query: GraphQLTaggedNode,
  variables: TOperation["variables"],
  /** Stable string for effect deps (JSON.stringify variables is fine). */
  varsKey: string
): QueryState<TOperation["response"]> {
  const [data, setData] = useState<TOperation["response"] | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    const sub = fetchQuery<TOperation>(environment, query, variables, {
      fetchPolicy: "store-or-network",
      networkCacheConfig: { force: false },
    }).subscribe({
      next: (value) => {
        if (cancelled) return;
        setData(value);
        setLoading(false);
      },
      error: (err: Error) => {
        if (cancelled) return;
        setError(err);
        setLoading(false);
      },
      complete: () => {
        if (cancelled) return;
        setLoading(false);
      },
    });

    return () => {
      cancelled = true;
      sub.unsubscribe();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- varsKey captures variables
  }, [query, varsKey]);

  return { data, error, loading };
}
