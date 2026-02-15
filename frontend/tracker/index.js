(() => {
  try {
    const {
      document,
      location,
      navigator,
      screen,
      history,
      sessionStorage,
      crypto,
    } = window;

    const scriptEl =
      document.currentScript || document.querySelector('script[src*="hk.js"]');
    if (!scriptEl) return;

    const collectDnt = scriptEl.getAttribute("data-collect-dnt") === "true";
    const disableBeacon =
      scriptEl.getAttribute("data-disable-beacon") === "true";

    const isBot = /bot|spider|crawl|slurp|ia_archiver/i.test(
      navigator.userAgent,
    );
    const isLocal =
      location.hostname === "localhost" || location.hostname === "127.0.0.1";
    const dntEnabled = navigator.doNotTrack === "1";

    if (isLocal || isBot || (dntEnabled && !collectDnt)) {
      window.hk = window.hk || {};
      window.hk.event = () => {};
      return;
    }

    const scriptUrl = new URL(scriptEl.src);
    const endpoint = `${scriptUrl.origin}/ingest`;

    const generateUUID = () => {
      if (crypto?.randomUUID) return crypto.randomUUID();
      return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, (c) =>
        (
          c ^
          (crypto.getRandomValues(new Uint8Array(1))[0] & (15 >> (c / 4)))
        ).toString(16),
      );
    };

    const SESSION_KEY = "hk_session";
    const SESSION_EXPIRY = 30 * 60 * 1000; // 30 minutes

    const getSessionId = () => {
      const now = Date.now();
      let sessionId = null;

      try {
        const stored = sessionStorage.getItem(SESSION_KEY);
        if (stored) {
          const [id, lastActive] = stored.split("|");
          if (now - parseInt(lastActive, 10) < SESSION_EXPIRY) {
            sessionId = id;
          }
        }
      } catch (e) {}

      if (!sessionId) {
        sessionId = generateUUID();
      }

      try {
        sessionStorage.setItem(SESSION_KEY, `${sessionId}|${now}`);
      } catch (e) {}

      return sessionId;
    };

    const sessionId = getSessionId();
    const initialReferrer = document.referrer;
    const initialHost = location.hostname;
    let lastPath = location.pathname;

    const readUtmValue = (params, key) => {
      const value = params.get(key);
      if (!value) return null;
      const trimmed = value.trim();
      return trimmed.length > 0 ? trimmed : null;
    };

    const sendPageView = () => {
      const currentPath = location.pathname;
      const searchParams = new URLSearchParams(location.search);

      try {
        sessionStorage.setItem(SESSION_KEY, `${sessionId}|${Date.now()}`);
      } catch (e) {}

      let referrer = initialReferrer;
      if (lastPath !== currentPath) {
        referrer = `${location.origin}${lastPath}`;
      }

      const isUnique =
        lastPath === currentPath &&
        referrer &&
        new URL(referrer).hostname !== initialHost;

      const payload = {
        path: currentPath,
        referrer: referrer || null,
        ua: navigator.userAgent,
        vp_w: window.innerWidth,
        vp_h: window.innerHeight,
        sc_w: screen.width,
        sc_h: screen.height,
        lang: navigator.language,
        u_src: readUtmValue(searchParams, "utm_source"),
        u_med: readUtmValue(searchParams, "utm_medium"),
        u_cmp: readUtmValue(searchParams, "utm_campaign"),
        u_trm: readUtmValue(searchParams, "utm_term"),
        u_cnt: readUtmValue(searchParams, "utm_content"),
        unique: !!isUnique,
        session_id: sessionId,
        page_id: generateUUID(),
      };

      const body = JSON.stringify(payload);
      const headers = { "Content-Type": "application/json" };

      // Use Beacon unless explicitly disabled via attribute
      if (navigator.sendBeacon && !disableBeacon) {
        const blob = new Blob([body], { type: "application/json" });
        navigator.sendBeacon(endpoint, blob);
      } else {
        fetch(endpoint, {
          method: "POST",
          body,
          headers,
          keepalive: true,
          credentials: "omit",
        }).catch(() => {});
      }

      lastPath = currentPath;
    };

    const patchHistory = (method) => {
      const original = history[method];
      return function (...args) {
        original.apply(this, args);
        sendPageView();
      };
    };

    history.pushState = patchHistory("pushState");
    history.replaceState = patchHistory("replaceState");

    window.addEventListener("popstate", sendPageView);
    window.addEventListener("hashchange", sendPageView);

    if (document.visibilityState === "prerender") {
      document.addEventListener(
        "visibilitychange",
        () => {
          if (document.visibilityState === "visible") sendPageView();
        },
        { once: true },
      );
    } else {
      sendPageView();
    }

    window.hk = window.hk || {};
    window.hk.event = (name, properties) => {
      const payload = {
        n: name,
        p: properties || {},
        sid: sessionId,
      };

      const body = JSON.stringify(payload);
      const eventEndpoint = `${scriptUrl.origin}/ingest/event`;

      if (navigator.sendBeacon && !disableBeacon) {
        const blob = new Blob([body], { type: "application/json" });
        navigator.sendBeacon(eventEndpoint, blob);
      } else {
        fetch(eventEndpoint, {
          method: "POST",
          body,
          headers: { "Content-Type": "application/json" },
          keepalive: true,
          credentials: "omit",
        }).catch(() => {});
      }
    };
  } catch (e) {
    if (console?.debug) console.debug("[HitKeep]", e);
  }
})();
