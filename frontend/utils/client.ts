import createClient from "openapi-fetch";
import type { paths } from "~/types/api";
export const createAPIClient = (baseUrl: string) =>
  createClient<paths>({ baseUrl, credentials: "same-origin" });
