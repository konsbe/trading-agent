export type MfeOperatorUiEvent = {
  event_origin?: string;
  event_type?: string;
  timestamp?: string;
  message?: any;
};

export const isMfeMetadataUpdateEvent = (evt: any): evt is MfeOperatorUiEvent => {
  return Boolean(
    evt &&
      typeof evt === 'object' &&
      ((evt as MfeOperatorUiEvent).event_type === 'mfeMetadataUpdate' ||
        (evt as MfeOperatorUiEvent).event_type === 'mfeUpdate' ||
        (evt as MfeOperatorUiEvent).event_type === 'ui_refresh')
  );
};
