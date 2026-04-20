import { bootstrapTracker } from './core';

(() => {
    try {
        bootstrapTracker();
    } catch (error) {
        if (console?.debug) {
            console.debug('[HitKeep]', error);
        }
    }
})();
