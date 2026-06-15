import { useEffect, useRef, useState } from 'react';
import { getToken } from '../api/client';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || '';

export interface OpsLiveSnapshot {
  rps: number;
  error_rate: number;
  p95_duration_ms: number;
  total_requests: number;
  goroutines: number;
  captured_at: string;
}

export type OpsStreamStatus = 'connecting' | 'live' | 'error';

// useOpsStream 通过 SSE（fetch 流式读取，携带 Authorization 头）订阅实时快照（M5）。
// enabled=false 时不连接。断线自动重连（3s 退避）。
export function useOpsStream(enabled: boolean) {
  const [snapshot, setSnapshot] = useState<OpsLiveSnapshot | null>(null);
  const [status, setStatus] = useState<OpsStreamStatus>('connecting');
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!enabled) {
      abortRef.current?.abort();
      return;
    }
    let stopped = false;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    const connect = async () => {
      const controller = new AbortController();
      abortRef.current = controller;
      setStatus('connecting');
      try {
        const res = await fetch(`${BASE_URL}/api/v1/admin/ops/stream`, {
          headers: { Authorization: `Bearer ${getToken() ?? ''}` },
          signal: controller.signal,
        });
        if (!res.ok || !res.body) throw new Error(`stream ${res.status}`);
        setStatus('live');
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          // 按 SSE 事件分隔（\n\n）解析
          let idx;
          while ((idx = buffer.indexOf('\n\n')) >= 0) {
            const chunk = buffer.slice(0, idx);
            buffer = buffer.slice(idx + 2);
            const line = chunk.split('\n').find((l) => l.startsWith('data:'));
            if (line) {
              try {
                setSnapshot(JSON.parse(line.slice(5).trim()));
              } catch {
                /* 忽略坏帧 */
              }
            }
          }
        }
        throw new Error('stream closed');
      } catch (err) {
        if (stopped || controller.signal.aborted) return;
        setStatus('error');
        retryTimer = setTimeout(connect, 3000);
      }
    };

    connect();
    return () => {
      stopped = true;
      if (retryTimer) clearTimeout(retryTimer);
      abortRef.current?.abort();
    };
  }, [enabled]);

  return { snapshot, status };
}
