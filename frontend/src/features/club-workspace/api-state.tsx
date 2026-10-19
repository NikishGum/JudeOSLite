import { useEffect, useRef, useState, type ReactNode } from "react";
import { Clock3, Inbox, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, apiGet } from "../../shared/api";
import type { List } from "./types";

export const errorMessage = (error: unknown) =>
  error instanceof ApiError
    ? error.message
    : "Не удалось связаться с клубом. Проверьте соединение и повторите действие.";
export function useResource<T>(
  path: `/api/${string}` | null,
  collection = true,
) {
  const [state, setState] = useState<{
    path: string | null;
    data: T | null;
    loading: boolean;
    unavailable: boolean;
    error: string;
  }>({ path, data: null, loading: !!path, unavailable: false, error: "" });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let controller: AbortController | undefined;
    function load() {
      controller?.abort();
      const current = new AbortController();
      controller = current;
      // A background read must not unmount editors or discard their drafts.
      setState((previous) => ({
        path,
        data: previous.path === path ? previous.data : null,
        loading: !!path && !(previous.path === path && previous.data),
        unavailable: false,
        error: "",
      }));
      if (!path) return;
      void apiGet<T>(path, controller.signal)
        .then((data) => {
          if (!current.signal.aborted)
            setState({
              path,
              data,
              loading: false,
              unavailable: false,
              error: "",
            });
        })
        .catch((error: unknown) => {
          if (current.signal.aborted) return;
          const unavailable =
            error instanceof ApiError &&
            (error.code === "module_unavailable" ||
              error.status === 501 ||
              (collection && error.status === 404));
          const denied =
            error instanceof ApiError &&
            [401, 403, 404, 410].includes(error.status);
          setState((previous) => ({
            path,
            data:
              denied || unavailable || previous.path !== path
                ? null
                : previous.data,
            loading: false,
            unavailable,
            error: unavailable ? "" : errorMessage(error),
          }));
        });
    }
    function revalidate() {
      if (document.visibilityState === "visible") load();
    }
    load();
    window.addEventListener("focus", revalidate);
    document.addEventListener("visibilitychange", revalidate);
    const timer = window.setInterval(revalidate, 30_000);
    return () => {
      controller?.abort();
      window.removeEventListener("focus", revalidate);
      document.removeEventListener("visibilitychange", revalidate);
      window.clearInterval(timer);
    };
  }, [path, attempt, collection]);
  return {
    ...(state.path === path
      ? state
      : { data: null, loading: !!path, unavailable: false, error: "" }),
    refresh: () => setAttempt((value) => value + 1),
  };
}
export function useCommand(onSuccess?: () => void) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const pending = useRef(false),
    mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  async function run<T>(action: () => Promise<T>, saved?: (value: T) => void) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      const result = await action();
      if (mounted.current) {
        saved?.(result);
        onSuccess?.();
        window.dispatchEvent(new Event("club-data-changed"));
      }
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    } finally {
      pending.current = false;
      if (mounted.current) setBusy(false);
    }
  }
  return { busy, error, run, clear: () => setError("") };
}
export function ResourceState({
  resource,
  empty = false,
  title = "Здесь пока ничего нет",
  children,
}: {
  resource: {
    loading: boolean;
    unavailable: boolean;
    error: string;
    refresh: () => void;
  };
  empty?: boolean;
  title?: string;
  children?: ReactNode;
}) {
  if (resource.loading)
    return (
      <div className="module-loading" role="status" aria-label="Загрузка">
        <Skeleton className="module-skeleton" />
        <Skeleton className="module-skeleton" />
        <Skeleton className="module-skeleton" />
        <span className="sr-only">Загружаем данные…</span>
      </div>
    );
  if (resource.unavailable)
    return (
      <div className="module-empty" role="status">
        <Clock3 aria-hidden="true" />
        <h3>Раздел готовится к запуску</h3>
        <p>Данные появятся после подключения этого раздела клубом.</p>
        <Button variant="outline" type="button" onClick={resource.refresh}>
          <RefreshCw />
          Проверить снова
        </Button>
      </div>
    );
  if (resource.error)
    return (
      <div className="module-empty" role="alert">
        <h3>Не удалось загрузить данные</h3>
        <p>{resource.error}</p>
        <Button variant="outline" type="button" onClick={resource.refresh}>
          Попробовать снова
        </Button>
      </div>
    );
  if (empty)
    return (
      <div className="module-empty">
        <Inbox aria-hidden="true" />
        <h3>{title}</h3>
        {children}
      </div>
    );
  return null;
}
export function Pagination({
  page,
  list,
  onPage,
}: {
  page: number;
  list: List<unknown> | null;
  onPage: (page: number) => void;
}) {
  if (!list || (!page && !list.hasMore)) return null;
  return (
    <nav className="module-pagination" aria-label="Страницы списка">
      <span>
        Страница {page + 1} · всего {list.total}
      </span>
      <Button
        variant="outline"
        disabled={!page}
        onClick={() => onPage(page - 1)}
      >
        Назад
      </Button>
      <Button
        variant="outline"
        disabled={!list.hasMore}
        onClick={() => onPage(page + 1)}
      >
        Далее
      </Button>
    </nav>
  );
}
