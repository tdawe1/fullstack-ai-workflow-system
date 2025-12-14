import { describe, expect, it } from 'vitest';

import { parseEventMessage, resolveEventLabel } from './events';

describe('parseEventMessage', () => {
  it('parses and normalises a valid event payload', () => {
    const payload = JSON.stringify({
      ts: '2024-01-01T00:00:00Z',
      status: 'running',
      type: 'heartbeat',
      message: 'Processing chunk 1',
      result: { foo: 'bar' },
    });

    const event = parseEventMessage(payload);
    expect(event).not.toBeNull();
    expect(event?.status).toBe('running');
    expect(event?.raw).toMatchObject({ type: 'heartbeat' });
    expect(event?.result).toEqual({ foo: 'bar' });
  });

  it('returns null for invalid JSON', () => {
    const event = parseEventMessage('{"status":');
    expect(event).toBeNull();
  });

  it('handles missing fields gracefully', () => {
    const event = parseEventMessage(JSON.stringify({}));
    expect(event).not.toBeNull();
    expect(event?.status).toBeUndefined();
    expect(event?.result).toBeNull();
  });
});

describe('resolveEventLabel', () => {
  const baseEvent = {
    raw: {},
  } as const;

  it('prefers status when defined', () => {
    expect(resolveEventLabel({ ...baseEvent, status: 'done' })).toBe('done');
  });

  it('falls back to type when status is missing', () => {
    expect(resolveEventLabel({ ...baseEvent, type: 'heartbeat' })).toBe('heartbeat');
  });

  it('returns "message" when message exists', () => {
    expect(resolveEventLabel({ ...baseEvent, message: 'hello' })).toBe('message');
  });

  it('defaults to update when no fields exist', () => {
    expect(resolveEventLabel({ ...baseEvent })).toBe('update');
  });
});
