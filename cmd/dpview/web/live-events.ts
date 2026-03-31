import { setConnectionState } from "./actions";
import {
  currentDataSchema,
  filesDataSchema,
  logDataSchema,
  seekDataSchema,
  settingsDataSchema,
} from "./contracts";
import type { Elements, State } from "./model";
import { renderConnectionBanner, renderStatus } from "./render";
import type { CurrentData, LogData, SettingsData } from "./types";
import { parseEventData } from "./validation";

interface LiveEventControllerOptions {
  elements: Elements;
  state: State;
  onFilesChanged: (data: typeof filesDataSchema._output) => void;
  onCurrentChanged: (data: CurrentData) => void;
  onPreviewUpdated: (data: CurrentData) => void;
  onSeekChanged: (data: typeof seekDataSchema._output | null) => void;
  onLogsChanged: (data: LogData) => void;
  onRenderStarted: (data: CurrentData) => void;
  onRenderFailed: (data: CurrentData) => void;
  onSettingsChanged: (data: SettingsData) => void;
  setClientError: (message: string) => void;
  clearClientError: () => void;
}

export interface LiveEventController {
  connect: () => void;
  close: () => void;
}

export function createLiveEventController(
  options: LiveEventControllerOptions
): LiveEventController {
  const { elements, state } = options;
  let eventSource: EventSource | null = null;
  let reconnectTimer = 0;
  let reconnectCountdownTimer = 0;

  function connect(): void {
    close();
    startEventStream(0);
  }

  function close(): void {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    window.clearTimeout(reconnectTimer);
    stopReconnectCountdown();
  }

  function startEventStream(attempt: number): void {
    let reconnectAttempt = attempt;
    setConnectionState(state, "connecting", attempt);
    renderStatus(elements, state);
    renderConnectionBanner(elements, state);

    const source = new EventSource("/events");
    eventSource = source;

    source.onopen = () => {
      if (eventSource !== source) {
        return;
      }
      state.bootstrapFailed = false;
      reconnectAttempt = 0;
      setConnectionState(state, "live", 0);
      options.clearClientError();
      renderStatus(elements, state);
      renderConnectionBanner(elements, state);
    };

    source.addEventListener("files_changed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onFilesChanged(
          parseEventData(event, "files_changed", filesDataSchema)
        );
      });
    });
    source.addEventListener("current_changed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onCurrentChanged(
          parseEventData(event, "current_changed", currentDataSchema)
        );
      });
    });
    source.addEventListener("preview_updated", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onPreviewUpdated(
          parseEventData(event, "preview_updated", currentDataSchema)
        );
      });
    });
    source.addEventListener("seek_changed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onSeekChanged(
          parseEventData(event, "seek_changed", seekDataSchema)
        );
      });
    });
    source.addEventListener("logs_changed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onLogsChanged(
          parseEventData(event, "logs_changed", logDataSchema)
        );
      });
    });
    source.addEventListener("render_started", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onRenderStarted(
          parseEventData(event, "render_started", currentDataSchema)
        );
      });
    });
    source.addEventListener("render_failed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onRenderFailed(
          parseEventData(event, "render_failed", currentDataSchema)
        );
      });
    });
    source.addEventListener("settings_changed", (event) => {
      handleEvent(source, reconnectAttempt, () => {
        options.onSettingsChanged(
          parseEventData(event, "settings_changed", settingsDataSchema)
        );
      });
    });
    source.onerror = () => {
      if (eventSource !== source) {
        return;
      }
      scheduleReconnect(source, reconnectAttempt + 1);
    };
  }

  function handleEvent(
    source: EventSource,
    attempt: number,
    handler: () => void
  ): void {
    if (eventSource !== source) {
      return;
    }
    try {
      handler();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Invalid live update payload.";
      options.setClientError(message);
      scheduleReconnect(source, attempt + 1);
    }
  }

  function scheduleReconnect(source: EventSource, attempt: number): void {
    if (eventSource !== source) {
      return;
    }
    source.close();
    if (eventSource === source) {
      eventSource = null;
    }
    const delay = reconnectDelayMs(attempt);
    const reconnectAt = Date.now() + delay;
    // Let the first automatic retry recover quietly before showing a full
    // disconnected banner for a longer outage.
    const connectionStatus = attempt > 1 ? "degraded" : "connecting";
    if (connectionStatus === "degraded") {
      options.setClientError("Live updates disconnected.");
    }
    setConnectionState(state, connectionStatus, attempt, reconnectAt);
    renderStatus(elements, state);
    renderConnectionBanner(elements, state);
    window.clearTimeout(reconnectTimer);
    startReconnectCountdown();
    reconnectTimer = window.setTimeout(() => {
      stopReconnectCountdown();
      startEventStream(attempt);
    }, delay);
  }

  function reconnectDelayMs(attempt: number): number {
    const base = 1000;
    const step = Math.max(0, attempt - 1);
    return Math.min(base * 2 ** step, 60_000);
  }

  function startReconnectCountdown(): void {
    stopReconnectCountdown();
    reconnectCountdownTimer = window.setInterval(() => {
      if (state.connectionStatus !== "degraded") {
        stopReconnectCountdown();
        return;
      }
      renderStatus(elements, state);
      renderConnectionBanner(elements, state);
    }, 1000);
  }

  function stopReconnectCountdown(): void {
    if (!reconnectCountdownTimer) {
      return;
    }
    window.clearInterval(reconnectCountdownTimer);
    reconnectCountdownTimer = 0;
  }

  return {
    connect,
    close,
  };
}
