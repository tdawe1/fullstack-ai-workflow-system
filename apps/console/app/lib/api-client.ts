/**
 * Global API Client with cookie-based authentication
 * 
 * All requests automatically include credentials (httpOnly cookies).
 * Handles token refresh on 401 responses.
 */

import { API_BASE } from './api';
import type {
  ProjectCodeResponse,
  ProjectSpecificationResponse,
  ProjectSummary,
  WorkflowApproveRequest,
  WorkflowGenerateRequest,
  WorkflowRefineRequest,
  WorkflowResult,
  WorkflowStatusResponse,
} from './workflow-types';

type ErrorPayload = {
  detail?: string;
  [key: string]: unknown;
};

export class APIError extends Error {
  constructor(
    message: string,
    public status: number,
    public response?: ErrorPayload,
    public correlationId?: string,
  ) {
    super(message);
    this.name = 'APIError';
  }
}

interface APIClientOptions extends RequestInit {
  skipRefresh?: boolean; // Skip auto-refresh on 401
}

let isRefreshing = false;
let refreshPromise: Promise<void> | null = null;

/**
 * Refresh access token using refresh token cookie
 */
async function refreshAccessToken(): Promise<void> {
  if (isRefreshing && refreshPromise) {
    // Wait for ongoing refresh
    return refreshPromise;
  }

  isRefreshing = true;
  refreshPromise = (async () => {
    try {
      const response = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        credentials: 'include', // Send refresh token cookie
      });

      if (!response.ok) {
        throw new Error('Token refresh failed');
      }

      // New tokens set as httpOnly cookies automatically
    } catch (error) {
      console.error('Failed to refresh token:', error);
      // Redirect to login if refresh fails
      if (typeof window !== 'undefined') {
        window.location.href = '/login';
      }
      throw error;
    } finally {
      isRefreshing = false;
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

async function buildAPIError(response: Response): Promise<APIError> {
  const correlationId = response.headers.get('X-Correlation-ID') ?? undefined;
  const errorData = (await response.json().catch(() => ({}))) as ErrorPayload;
  const message = errorData.detail ?? `HTTP ${response.status}`;
  return new APIError(message, response.status, errorData, correlationId);
}

/**
 * Global API client with automatic token refresh
 * 
 * Features:
 * - Automatically includes credentials (httpOnly cookies)
 * - Auto-refreshes token on 401 responses
 * - Retries original request after refresh
 * - Consistent error handling
 * 
 * @example
 * ```typescript
 * const user = await apiClient('/auth/me');
 * const projects = await apiClient('/projects', { method: 'POST', body: data });
 * ```
 */
export async function apiClient<T = any>(
  url: string,
  options: APIClientOptions = {}
): Promise<T> {
  const { skipRefresh = false, ...fetchOptions } = options;

  // Always include credentials for cookie-based auth
  const requestOptions: RequestInit = {
    ...fetchOptions,
    credentials: 'include',
  };

  // Build candidate bases for resilience in local dev
  const bases: string[] = [];
  bases.push(API_BASE);
  // In dev, if API_BASE points to 8000 or network fails, try proxy and 8001
  if (typeof window !== 'undefined') {
    if (API_BASE.startsWith('http://localhost:8000')) {
      bases.push('/api', 'http://localhost:8001');
    } else if (API_BASE === '/api') {
      bases.push('http://localhost:8001');
    } else {
      bases.push('/api');
    }
  }

  const buildUrl = (base: string, path: string) => (path.startsWith('http') ? path : `${base}${path}`);

  let lastNetworkError: unknown = null;
  let response: Response | null = null;
  let attemptedUrl = '';

  // Try candidates until one responds (avoids stale 8000 configs)
  for (const base of bases) {
    const candidate = buildUrl(base, url);
    attemptedUrl = candidate;
    try {
      response = await fetch(candidate, requestOptions);
      lastNetworkError = null;
      break;
    } catch (e: any) {
      // Network error (e.g., ERR_CONNECTION_REFUSED)
      lastNetworkError = e;
      continue; // Try next base
    }
  }

  if (!response) {
    // All attempts failed with network error
    throw new APIError(
      lastNetworkError instanceof Error ? lastNetworkError.message : 'Network error',
      0,
    );
  }

  try {
    
    // Handle 401 Unauthorized (token expired)
    if (response.status === 401 && !skipRefresh) {
      console.log('Token expired, refreshing...');

      try {
        // Refresh token
        await refreshAccessToken();

        // Retry original request
        const retryResponse = await fetch(fullUrl, {
          ...requestOptions,
          // Skip refresh on retry to avoid infinite loop
        });

        if (!retryResponse.ok) {
          throw await buildAPIError(retryResponse);
        }

        const retryContentType = retryResponse.headers.get('content-type');
        if (retryContentType?.includes('application/json')) {
          return await retryResponse.json();
        }

        return retryResponse as unknown as T;
      } catch (refreshError) {
        // Refresh failed, redirect to login
        if (typeof window !== 'undefined') {
          window.location.href = '/login';
        }
        throw refreshError;
      }
    }

    // Handle other errors
    if (!response.ok) {
      throw await buildAPIError(response);
    }

    // Parse JSON response
    const contentType = response.headers.get('content-type');
    if (contentType?.includes('application/json')) {
      return await response.json();
    }

    // Return raw response for non-JSON
    return response as any;
  } catch (error) {
    if (error instanceof APIError) {
      throw error;
    }

    // Network error or other fetch error
    throw new APIError(
      error instanceof Error ? error.message : 'Network error',
      0,
    );
  }
}

/**
 * Get temporary WebSocket token for Terminal
 * 
 * WebSockets don't support httpOnly cookies in all scenarios.
 * This gets a short-lived (5 min) token specifically for WS auth.
 */
export async function getWebSocketToken(): Promise<string> {
  try {
    const response = await apiClient<{ ws_token: string }>('/auth/ws-token', {
      method: 'POST',
    });
    return response.ws_token;
  } catch (error) {
    console.error('Failed to get WebSocket token:', error);
    throw error;
  }
}

/**
 * Convenience methods for common HTTP methods
 */
export const api = {
  get: <T = any>(url: string, options?: APIClientOptions) =>
    apiClient<T>(url, { ...options, method: 'GET' }),

  post: <T = any>(url: string, data?: any, options?: APIClientOptions) =>
    apiClient<T>(url, {
      ...options,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      body: data ? JSON.stringify(data) : undefined,
    }),

  put: <T = any>(url: string, data?: any, options?: APIClientOptions) =>
    apiClient<T>(url, {
      ...options,
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      body: data ? JSON.stringify(data) : undefined,
    }),

  delete: <T = any>(url: string, options?: APIClientOptions) =>
    apiClient<T>(url, { ...options, method: 'DELETE' }),

  patch: <T = any>(url: string, data?: any, options?: APIClientOptions) =>
    apiClient<T>(url, {
      ...options,
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      body: data ? JSON.stringify(data) : undefined,
  }),
};

export const apiFetch = apiClient;

const jsonHeaders = {
  'Content-Type': 'application/json',
};

export async function generateProject(
  projectId: string,
  payload: WorkflowGenerateRequest,
): Promise<WorkflowResult> {
  return apiClient<WorkflowResult>(`/projects/${projectId}/generate`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}

export async function approveSpecification(
  projectId: string,
  payload: WorkflowApproveRequest,
): Promise<WorkflowResult> {
  return apiClient<WorkflowResult>(`/projects/${projectId}/approve`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}

export async function regenerateProject(
  projectId: string,
  payload: WorkflowRefineRequest,
): Promise<WorkflowResult> {
  return apiClient<WorkflowResult>(`/projects/${projectId}/regenerate`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}

export async function getProjectSpecification(projectId: string): Promise<ProjectSpecificationResponse> {
  return apiClient<ProjectSpecificationResponse>(`/projects/${projectId}/specification`);
}

export async function getProjectCode(projectId: string): Promise<ProjectCodeResponse> {
  return apiClient<ProjectCodeResponse>(`/projects/${projectId}/code`);
}

export async function getWorkflowStatus(projectId: string): Promise<WorkflowStatusResponse> {
  return apiClient<WorkflowStatusResponse>(`/projects/${projectId}/status`);
}

export async function listProjects(): Promise<ProjectSummary[]> {
  return apiClient<ProjectSummary[]>('/projects');
}

export async function createProject(payload: { name: string; description?: string }): Promise<ProjectSummary> {
  return apiClient<ProjectSummary>('/projects', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}
