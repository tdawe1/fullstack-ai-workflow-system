import { describe, expect, it } from 'vitest';

import { buildWsUrlFromHttp, withTokenQuery } from './api';

describe('withTokenQuery', () => {
  it('leaves the URL untouched when no token is provided', () => {
    expect(withTokenQuery('http://localhost/resource', null)).toBe('http://localhost/resource');
  });

  it('appends the token as a query parameter when none exist', () => {
    expect(withTokenQuery('https://api.example.com/data', 'secret-token')).toBe(
      'https://api.example.com/data?token=secret-token',
    );
  });

  it('preserves existing query parameters with an ampersand separator', () => {
    expect(withTokenQuery('https://api.example.com/data?filter=latest', 'special token')).toBe(
      'https://api.example.com/data?filter=latest&token=special%20token',
    );
  });
});

describe('buildWsUrlFromHttp', () => {
  it('converts http to ws and appends path', () => {
    expect(buildWsUrlFromHttp('http://localhost:8001', '/ws/terminal')).toBe(
      'ws://localhost:8001/ws/terminal',
    );
  });

  it('converts https to wss', () => {
    expect(buildWsUrlFromHttp('https://api.example.com', '/socket')).toBe(
      'wss://api.example.com/socket',
    );
  });

  it('falls back gracefully on invalid base URLs', () => {
    expect(buildWsUrlFromHttp('http://localhost:8001/', 'socket')).toBe(
      'ws://localhost:8001/socket',
    );
  });
});
