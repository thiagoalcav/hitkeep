(() => {
  try {
    const { document, location, navigator, screen, history, sessionStorage, crypto } = window;

    // 1. Filter Out Unwanted Traffic Early
    // We do this before any logic to ensure we don't write to storage if DNT is active.
    const scriptEl = document.currentScript || document.querySelector('script[src*="hk.js"]');
    if (!scriptEl) return;

    const collectDnt = scriptEl.getAttribute('data-collect-dnt') === 'true';
    const isBot = /bot|spider|crawl|slurp|ia_archiver/i.test(navigator.userAgent);
    const isLocal = location.hostname === 'localhost' || location.hostname === '127.0.0.1';
    const dntEnabled = navigator.doNotTrack === '1';

    if (isLocal || isBot || (dntEnabled && !collectDnt)) {
      return;
    }

    // 2. Configuration & Endpoint
    const scriptUrl = new URL(scriptEl.src);
    const endpoint = `${scriptUrl.origin}/ingest`;

    // 3. Helper: Generate UUID (v4)
    const generateUUID = () => {
      if (crypto?.randomUUID) return crypto.randomUUID();
      return '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, c =>
        (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
      );
    };

    // 4. Session Management (Persist across reloads)
    const SESSION_KEY = 'hk_session';
    const SESSION_EXPIRY = 30 * 60 * 1000; // 30 minutes in milliseconds

    const getSessionId = () => {
      const now = Date.now();
      let sessionId = null;

      try {
        const stored = sessionStorage.getItem(SESSION_KEY);
        if (stored) {
          const [id, lastActive] = stored.split('|');
          // If the session was active within the last 30 minutes, keep it.
          if (now - parseInt(lastActive, 10) < SESSION_EXPIRY) {
            sessionId = id;
          }
        }
      } catch (e) {
        // Storage might be blocked
      }

      if (!sessionId) {
        sessionId = generateUUID();
      }

      // Update the timestamp to keep the session alive
      try {
        sessionStorage.setItem(SESSION_KEY, `${sessionId}|${now}`);
      } catch (e) {}

      return sessionId;
    };

    const sessionId = getSessionId();
    const initialReferrer = document.referrer;
    const initialHost = location.hostname;
    let lastPath = location.pathname;

    // 5. Send Data
    const sendPageView = () => {
      const currentPath = location.pathname;
      
      // Update session timestamp on activity
      try {
        sessionStorage.setItem(SESSION_KEY, `${sessionId}|${Date.now()}`);
      } catch (e) {}

      // Calculate Referrer:
      // If we just navigated inside the SPA (currentPath !== lastPath), the referrer is the *previous* path on this site.
      // If it's the first load, the referrer is document.referrer (external or empty).
      let referrer = initialReferrer;
      if (lastPath !== currentPath) {
        referrer = `${location.origin}${lastPath}`;
      }

      // Unique Visit Check:
      // Strictly true only if coming from a different hostname and this is the first hit of the session state.
      // (Refined logic: Only the entry point of the session should be marked unique=true)
      const isUnique = lastPath === currentPath && referrer && new URL(referrer).hostname !== initialHost;

      const payload = {
        path: currentPath,
        referrer: referrer || null,
        ua: navigator.userAgent,
        vp_w: window.innerWidth,
        vp_h: window.innerHeight,
        sc_w: screen.width,
        sc_h: screen.height,
        lang: navigator.language,
        unique: !!isUnique,
        session_id: sessionId,
        page_id: generateUUID(),
      };

      const body = JSON.stringify(payload);
      const headers = { 'Content-Type': 'application/json' };

      if (navigator.sendBeacon) {
        const blob = new Blob([body], { type: 'application/json' });
        navigator.sendBeacon(endpoint, blob);
      } else {
        fetch(endpoint, {
          method: 'POST',
          body,
          headers,
          keepalive: true,
          credentials: 'omit',
        }).catch(() => {}); // Silent fail
      }

      lastPath = currentPath;
    };

    // 6. SPA Navigation Patches
    const patchHistory = (method) => {
      const original = history[method];
      return function (...args) {
        original.apply(this, args);
        sendPageView();
      };
    };

    history.pushState = patchHistory('pushState');
    history.replaceState = patchHistory('replaceState');

    window.addEventListener('popstate', sendPageView);
    window.addEventListener('hashchange', sendPageView);

    // 7. Initial Trigger
    if (document.visibilityState === 'prerender') {
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') sendPageView();
      }, { once: true });
    } else {
      sendPageView();
    }

  } catch (e) {
    if (console?.debug) console.debug('[HitKeep]', e);
  }
})();