"use client";

import { type ReactNode, useState } from "react";
import { usePathname } from "next/navigation";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClientError } from "@webtui/api-client";
import { ThemeProvider } from "@webtui/ui";
import { AuthProvider } from "@/features/auth/auth-provider";

export function AppProviders({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            gcTime: 10 * 60_000,
            networkMode: "offlineFirst",
            refetchOnReconnect: "always",
            refetchOnWindowFocus: false,
            retry: retryTransientQuery,
            staleTime: 30_000
          }
        }
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        {pathname === "/join" || pathname.startsWith("/join/")
          ? children
          : <AuthProvider>{children}</AuthProvider>}
      </ThemeProvider>
    </QueryClientProvider>
  );
}

function retryTransientQuery(failureCount: number, error: Error) {
  if (failureCount >= 2) {
    return false;
  }

  if (error instanceof ApiClientError) {
    return error.status === 408 || error.status === 429 || error.status >= 500;
  }

  return true;
}
