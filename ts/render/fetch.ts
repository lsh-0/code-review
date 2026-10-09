// the GUI's fetch control: fetch the feedback on the branch's pull request, or on
// the pull request a pasted URL names, and report the outcome in the import banner.
//
// The flow takes everything it touches as parameters, so a DOM test drives it with
// a stubbed bridge and no Wails runtime.

import type { ImportResult } from "../client.ts";

// what the fetch flow needs from the rest of the GUI.
export interface FetchDeps {
  // runs the fetch in the backend. An empty url means the branch's pull request.
  fetch: (url: string) => Promise<ImportResult>;
  // reloads review state and re-renders, so fetched feedback appears.
  reload: () => Promise<void>;
  // shows report lines, or one failure message, in the import banner.
  report: (lines: string[]) => void;
  // signals success on the button.
  flash: (button: HTMLElement) => void;
}

// the message a rejected bridge call carries. Wails rejects with the Go error's text
// as a string, and a bridge failure inside the client throws an `Error`.
export function failureMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

// fetches feedback for the URL in `input`, or for the branch's pull request when it
// is blank. The button is disabled until the fetch settles, so a second click cannot
// start a second fetch. On success the URL is cleared, review state reloads, and the
// report shows. On failure the message shows and nothing is reloaded.
export async function triggerFetch(
  button: HTMLButtonElement,
  input: HTMLInputElement,
  deps: FetchDeps,
): Promise<void> {
  if (button.disabled) {
    return;
  }
  button.disabled = true;
  try {
    const result = await deps.fetch(input.value.trim());
    input.value = "";
    await deps.reload();
    deps.flash(button);
    deps.report(result.lines);
  } catch (error) {
    deps.report([`Fetch failed: ${failureMessage(error)}`]);
  } finally {
    button.disabled = false;
  }
}
