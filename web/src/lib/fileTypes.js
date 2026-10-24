

export const EXPECTED_FILENAMES = {
  Localization: 'Localization.csv',
  Dialogues: 'Dialogues.csv',
  PDA: 'PDA.csv',
};

const EXTRA_EXPECTED_FILENAMES = {
  LocalizationVanilla: 'Localization.csv',
};

export function expectedFilenameFor(fileType) {
  return EXPECTED_FILENAMES[fileType] ?? EXTRA_EXPECTED_FILENAMES[fileType];
}

const FILE_TYPE_LABELS = {
  LocalizationVanilla: 'Localization (vanilla)',
};

export function fileTypeLabel(fileType) {
  return FILE_TYPE_LABELS[fileType] ?? fileType;
}
