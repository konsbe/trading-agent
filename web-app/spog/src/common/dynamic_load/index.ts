import { loadRemoteEntry } from './loadRemoteEntry';
import { loadMfeComponent } from './loadMfeComponent';
import { loadAppConfig, isMfeEnabled, getMFE, getIframeEntry, isIframeEnabled } from './config';
import { clearAllMfeCaches, clearMfeCachesForScope } from './mfeCache';

export {
	loadMfeComponent,
	loadRemoteEntry,
	isMfeEnabled,
	loadAppConfig,
	getMFE,
	getIframeEntry,
	isIframeEnabled,
	clearAllMfeCaches,
	clearMfeCachesForScope,
};
