export type EventMessage = {
  ts?: string;
  status?: string;
  message?: string;
  type?: string;
  result?: Record<string, unknown> | null;
  raw: Record<string, unknown>;
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const asString = (value: unknown) => (typeof value === 'string' ? value : undefined);

export const parseEventMessage = (payload: string): EventMessage | null => {
  try {
    const parsed = JSON.parse(payload) as unknown;
    if (!isRecord(parsed)) {
      return null;
    }

    const resultCandidate = parsed.result;
    const result = isRecord(resultCandidate) ? resultCandidate : null;

    return {
      ts: asString(parsed.ts),
      status: asString(parsed.status),
      message: asString(parsed.message),
      type: asString(parsed.type),
      result,
      raw: parsed,
    };
  } catch (error) {
    console.error('Failed to parse event payload', error);
    return null;
  }
};

export const resolveEventLabel = (event: EventMessage) => {
  if (event.status && event.status.trim().length > 0) {
    return event.status;
  }
  if (event.type && event.type.trim().length > 0) {
    return event.type;
  }
  if (event.message && event.message.trim().length > 0) {
    return 'message';
  }
  return 'update';
};
