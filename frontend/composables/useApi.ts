import { createAPIClient } from "~/utils/client";
export const useApi = () => {
  const config = useRuntimeConfig();
  const typed = createAPIClient(
    import.meta.server ? config.apiBase + "/api/v1" : "/api/v1",
  );
  const catalog = async () => {
    const { data, error } = await typed.GET("/dictionaries");
    if (error) throw new Error(error.error);
    return data!;
  };
  const csrf = useState<string>("csrf", () => "");
  const fetcher = useRequestFetch();
  const call = async <T = any>(path: string, options: any = {}): Promise<T> => {
    try {
      return await fetcher<T>("/api/v1" + path, {
        ...options,
        headers: { ...options.headers, "X-CSRF-Token": csrf.value },
      });
    } catch (error: any) {
      throw createError({
        statusCode: error.statusCode || 503,
        message:
          error.data?.error || "Nie udało się połączyć. Spróbuj ponownie.",
      });
    }
  };
  const session = async () => {
    const u = await call("/auth/me");
    csrf.value = u.csrf;
    return u;
  };
  return { call, session, csrf, catalog };
};
