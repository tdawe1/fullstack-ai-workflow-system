'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { API_BASE, withTokenQuery } from './api';
import { APIError, apiFetch } from './api-client';
import { parseEventMessage } from './events';
import type { EventMessage } from './events';

export type CrewRunResponse = {
  id: string;
  crew_id: string;
  status: string;
  input: Record<string, unknown>;
  result?: Record<string, unknown> | null;
};

export function useRunManager(token: string | null) {
  const [crewId, setCrewId] = useState('spec_to_tasks');
  const [prompt, setPrompt] = useState('');
  const [run, setRun] = useState<CrewRunResponse | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [events, setEvents] = useState<EventMessage[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const eventSourceRef = useRef<EventSource | null>(null);

  const isRunning = useMemo(() => status === 'queued' || status === 'running', [status]);

  useEffect(() => {
    return () => {
      eventSourceRef.current?.close();
    };
  }, []);

  useEffect(() => {
    if (!run?.id) {
      return;
    }

    eventSourceRef.current?.close();
    const streamUrl = withTokenQuery(`${API_BASE}/crews/runs/${run.id}/events`, token);
    const source = new EventSource(streamUrl);
    eventSourceRef.current = source;

    source.onmessage = (evt) => {
      if (!evt.data) {
        return;
      }

      const eventMessage = parseEventMessage(evt.data);
      if (!eventMessage) {
        return;
      }

      setEvents((prev) => [...prev, eventMessage]);

      if (eventMessage.status) {
        setStatus(eventMessage.status);
      }

      if (eventMessage.result) {
        setRun((prev) => (prev ? { ...prev, result: eventMessage.result } : prev));
      }
    };

    source.onerror = () => {
      source.close();
    };

    return () => {
      source.close();
    };
  }, [run?.id, token]);

  const submitRun = useCallback(async () => {
    setIsSubmitting(true);
    setError(null);
    setEvents([]);
    setStatus(null);
    setRun(null);

    try {
      const data = await apiFetch<CrewRunResponse>('/crews/runs', {
        method: 'POST',
        body: JSON.stringify({
          crew_id: crewId,
          input: { prompt },
        }),
      });
      setRun(data);
      setStatus(data.status);
    } catch (err) {
      if (err instanceof APIError) {
        const correlationSuffix = err.correlationId ? `\nCorrelation ID: ${err.correlationId}` : '';
        setError(`${err.message}${correlationSuffix}`);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Unable to start the crew run.');
      }
    } finally {
      setIsSubmitting(false);
    }
  }, [crewId, prompt, token]);

  const cancelRun = useCallback(async () => {
    if (!run?.id) {
      return;
    }

    try {
      await apiFetch(`/crews/runs/${run.id}/cancel`, {
        method: 'POST',
        body: JSON.stringify({ reason: 'User requested cancel' }),
      });
    } catch (err) {
      console.error('Failed to cancel run', err);
    }
  }, [run?.id, token]);

  const formattedInput = useMemo(
    () => JSON.stringify(run?.input ?? {}, null, 2),
    [run?.input],
  );

  const formattedResult = useMemo(
    () => (run?.result ? JSON.stringify(run.result, null, 2) : null),
    [run?.result],
  );

  return {
    crewId,
    setCrewId,
    prompt,
    setPrompt,
    run,
    status,
    events,
    isSubmitting,
    error,
    isRunning,
    formattedInput,
    formattedResult,
    submitRun,
    cancelRun,
  };
}
