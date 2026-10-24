

let focusedId = $state(null);

export function focusedAIJob() {
  return focusedId;
}

export function focusAIJob(id) {
  focusedId = id;
}

export function clearAIJobFocusIfMatches(id) {
  if (focusedId === id) focusedId = null;
}
