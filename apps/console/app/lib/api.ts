'use client';

// IMPORTANT: Use /api for proxy, falls back to direct API on 8001
export const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, '') || '/api';

export const ACCESS_TOKEN_STORAGE_KEY = 'kyros.access_token';

export const buildWsUrlFromHttp = (httpUrl: string, path: string) => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  try {
    const derived = new URL(httpUrl);
    derived.protocol = derived.protocol === 'https:' ? 'wss:' : 'ws:';
    derived.pathname = normalizedPath;
    derived.search = '';
    derived.hash = '';
    return derived.toString();
  } catch {
    const fallback = httpUrl.replace(/^http/, 'ws').replace(/\/$/, '');
    return `${fallback}${normalizedPath}`;
  }
};

export const TERMINAL_WS_URL =
  process.env.NEXT_PUBLIC_TERMINAL_WS_URL?.replace(/\/$/, '') ||
  buildWsUrlFromHttp(API_BASE, '/ws/terminal');

export const withTokenQuery = (url: string, token: string | null) => {
  if (!token) {
    return url;
  }
  const separator = url.includes('?') ? '&' : '?';
  return `${url}${separator}token=${encodeURIComponent(token)}`;
};
