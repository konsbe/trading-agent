import { isMfeMetadataUpdateEvent } from './mfeOperatorEvents';

describe('isMfeMetadataUpdateEvent', () => {
  it('returns true only for object events with event_type=mfeMetadataUpdate', () => {
    expect(isMfeMetadataUpdateEvent({ event_type: 'mfeMetadataUpdate' })).toBe(true);
    expect(isMfeMetadataUpdateEvent({ event_type: 'mfeMetadataUpdate', event_origin: 'x' })).toBe(true);
    expect(isMfeMetadataUpdateEvent({ event_type: 'ui_refresh' })).toBe(true);
    expect(isMfeMetadataUpdateEvent({ event_type: 'mfeUpdate' })).toBe(true);

    expect(isMfeMetadataUpdateEvent(null)).toBe(false);
    expect(isMfeMetadataUpdateEvent(undefined)).toBe(false);
    expect(isMfeMetadataUpdateEvent('mfeMetadataUpdate')).toBe(false);
    expect(isMfeMetadataUpdateEvent(123)).toBe(false);
    expect(isMfeMetadataUpdateEvent({})).toBe(false);
    expect(isMfeMetadataUpdateEvent({ event_type: 'other' })).toBe(false);
  });
});
