import { vi } from 'vitest';

import { bootstrapTracker, classifyFormSubmit, classifyLinkEvent, hasExplicitTrackingTag, readTrackerOptions, sanitizeTrackedUrl } from './core';

type ListenerMap = Record<string, EventListener[]>;

function okResponse(): Response {
    return new Response('', { status: 202 });
}

function flushPromises(): Promise<void> {
    return Promise.resolve().then(() => undefined);
}

function fetchMockOk() {
    return vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        void input;
        void init;
        return Promise.resolve(okResponse());
    });
}

function trackerHarness(path = '/signup?utm_source=launch&utm_medium=email') {
    const script = document.createElement('script');
    script.src = 'https://analytics.example.com/hk.js';

    let currentUrl = new URL(path, 'https://app.example.com');
    const location = {
        get href() {
            return currentUrl.href;
        },
        get hostname() {
            return currentUrl.hostname;
        },
        get pathname() {
            return currentUrl.pathname;
        },
        get search() {
            return currentUrl.search;
        },
        get origin() {
            return currentUrl.origin;
        }
    } as Location;

    const windowListeners: ListenerMap = {};
    const documentListeners: ListenerMap = {};
    let visibilityState = 'visible';
    const stored = new Map<string, string>();
    const sessionStorage = {
        getItem: vi.fn((key: string) => stored.get(key) ?? null),
        setItem: vi.fn((key: string, value: string) => {
            stored.set(key, value);
        })
    };
    const sendBeacon = vi.fn((url: string | URL, data?: BodyInit | null) => {
        void url;
        void data;
        return true;
    });
    const history = {
        pushState: vi.fn((_state: unknown, _title: string, url?: string | URL | null) => {
            if (url) {
                currentUrl = new URL(url, currentUrl.href);
            }
        }),
        replaceState: vi.fn((_state: unknown, _title: string, url?: string | URL | null) => {
            if (url) {
                currentUrl = new URL(url, currentUrl.href);
            }
        })
    };
    const fakeDocument = {
        currentScript: script,
        referrer: 'https://referrer.example.com/article?secret=1',
        get visibilityState() {
            return visibilityState;
        },
        querySelector: vi.fn(() => script),
        addEventListener: vi.fn((name: string, listener: EventListener) => {
            documentListeners[name] = [...(documentListeners[name] ?? []), listener];
        })
    };
    const win = {
        document: fakeDocument,
        location,
        navigator: {
            userAgent: 'Mozilla/5.0',
            doNotTrack: '0',
            language: 'en-US',
            sendBeacon
        },
        screen: { width: 1440, height: 900 },
        history,
        sessionStorage,
        crypto: {
            randomUUID: vi.fn(() => '10000000-0000-4000-8000-000000000001'),
            getRandomValues: crypto.getRandomValues.bind(crypto)
        },
        innerWidth: 1280,
        innerHeight: 720,
        addEventListener: vi.fn((name: string, listener: EventListener) => {
            windowListeners[name] = [...(windowListeners[name] ?? []), listener];
        }),
        hk: undefined
    } as unknown as Window & typeof globalThis;

    return {
        documentListeners,
        history,
        script,
        sendBeacon,
        sessionStorage,
        setVisibilityState: (next: string) => {
            visibilityState = next;
        },
        win,
        windowListeners
    };
}

describe('tracker core', () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('classifies outbound links with privacy-safe properties', () => {
        const link = document.createElement('a');
        link.href = 'https://external.example.com/docs/pricing?plan=pro#cta';

        const trackedEvent = classifyLinkEvent(link, new URL('https://app.example.com/pricing'));

        expect(trackedEvent).toEqual({
            name: 'outbound_click',
            properties: {
                target_host: 'external.example.com',
                target_path: '/docs/pricing',
                target_protocol: 'https'
            }
        });
    });

    it('classifies same-origin downloads and strips query strings', () => {
        const link = document.createElement('a');
        link.href = 'https://app.example.com/files/report.csv?token=abc#download';

        const trackedEvent = classifyLinkEvent(link, new URL('https://app.example.com/dashboard'));

        expect(trackedEvent).toEqual({
            name: 'file_download',
            properties: {
                file_host: 'app.example.com',
                file_path: '/files/report.csv',
                file_ext: 'csv'
            }
        });
    });

    it('prefers outbound tracking over download tracking for external files', () => {
        const link = document.createElement('a');
        link.href = 'https://cdn.example.net/files/report.pdf?token=abc';
        link.setAttribute('download', '');

        const trackedEvent = classifyLinkEvent(link, new URL('https://app.example.com/dashboard'));

        expect(trackedEvent?.name).toBe('outbound_click');
    });

    it('does not auto-track links inside forms', () => {
        const form = document.createElement('form');
        const link = document.createElement('a');
        link.href = 'https://external.example.com/docs';
        form.append(link);

        const trackedEvent = classifyLinkEvent(link, new URL('https://app.example.com/contact'));

        expect(trackedEvent).toBeNull();
    });

    it('extracts privacy-safe form submit properties', () => {
        const form = document.createElement('form');
        form.id = 'newsletter-form';
        form.method = 'POST';
        form.action = 'https://forms.example.com/submit?source=landing#done';

        const trackedEvent = classifyFormSubmit(form, new URL('https://app.example.com/blog/post'));

        expect(trackedEvent).toEqual({
            name: 'form_submit',
            properties: {
                action_host: 'forms.example.com',
                action_path: '/submit',
                method: 'post',
                same_origin: false,
                form_id: 'newsletter-form'
            }
        });
    });

    it('defaults form actions to the current page when action is missing', () => {
        const form = document.createElement('form');

        const trackedEvent = classifyFormSubmit(form, new URL('https://app.example.com/signup?step=2#account'));

        expect(trackedEvent).toEqual({
            name: 'form_submit',
            properties: {
                action_host: 'app.example.com',
                action_path: '/signup',
                method: 'get',
                same_origin: true
            }
        });
    });

    it('skips auto-tracking when an explicit tracking tag is present', () => {
        const wrapper = document.createElement('div');
        wrapper.setAttribute('data-hk-event', 'purchase_started');

        const link = document.createElement('a');
        link.href = 'https://external.example.com/docs';
        wrapper.append(link);

        const trackedEvent = classifyLinkEvent(link, new URL('https://app.example.com/docs'));

        expect(hasExplicitTrackingTag(link)).toBe(true);
        expect(trackedEvent).toBeNull();
    });

    it('reads auto-tracking opt-out flags from the snippet element', () => {
        const script = document.createElement('script');
        script.setAttribute('data-collect-dnt', 'true');
        script.setAttribute('data-disable-beacon', 'true');
        script.setAttribute('data-disable-spa-tracking', 'true');
        script.setAttribute('data-disable-outbound-tracking', 'true');
        script.setAttribute('data-disable-download-tracking', 'true');
        script.setAttribute('data-disable-form-tracking', 'true');

        expect(readTrackerOptions(script)).toEqual({
            collectDnt: true,
            disableBeacon: true,
            disableSpaTracking: true,
            disableOutboundTracking: true,
            disableDownloadTracking: true,
            disableFormTracking: true
        });
    });

    it('ignores unsupported link protocols', () => {
        const link = document.createElement('a');
        link.href = 'mailto:hello@example.com';

        expect(classifyLinkEvent(link, new URL('https://app.example.com'))).toBeNull();
        expect(sanitizeTrackedUrl('javascript:void(0)', 'https://app.example.com')).toBeNull();
    });

    it('falls back to keepalive fetch when sendBeacon cannot queue the hit', () => {
        const fetchMock = fetchMockOk();
        vi.stubGlobal('fetch', fetchMock);
        const harness = trackerHarness();
        harness.sendBeacon.mockReturnValue(false);

        bootstrapTracker(harness.win);

        expect(harness.sendBeacon).toHaveBeenCalledTimes(1);
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock.mock.calls[0]?.[0]).toBe('https://analytics.example.com/ingest');
        const fetchInit = fetchMock.mock.calls[0]?.[1];
        expect(fetchInit?.keepalive).toBe(true);
        expect(fetchInit?.credentials).toBe('omit');
    });

    it('queues failed requests in memory and flushes them on pagehide', async () => {
        const fetchMock = fetchMockOk();
        fetchMock.mockRejectedValueOnce(new Error('offline'));
        vi.stubGlobal('fetch', fetchMock);
        const harness = trackerHarness();
        harness.script.setAttribute('data-disable-beacon', 'true');

        bootstrapTracker(harness.win);
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledTimes(1);
        harness.windowListeners['pagehide']?.[0]?.(new Event('pagehide'));
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('flushes queued requests when the document becomes hidden', async () => {
        const fetchMock = fetchMockOk();
        fetchMock.mockRejectedValueOnce(new Error('offline'));
        vi.stubGlobal('fetch', fetchMock);
        const harness = trackerHarness();
        harness.script.setAttribute('data-disable-beacon', 'true');

        bootstrapTracker(harness.win);
        await flushPromises();

        harness.setVisibilityState('hidden');
        harness.documentListeners['visibilitychange']?.[0]?.(new Event('visibilitychange'));
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('keeps the in-memory retry queue bounded', async () => {
        const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
            void input;
            void init;
            return Promise.reject(new Error('offline'));
        });
        vi.stubGlobal('fetch', fetchMock);
        const harness = trackerHarness();
        harness.script.setAttribute('data-disable-beacon', 'true');

        bootstrapTracker(harness.win);
        for (let index = 0; index < 12; index += 1) {
            (harness.win as Window & typeof globalThis & { hk?: { event?: (name: string) => void } }).hk?.event?.(`queued_${index}`);
        }
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledTimes(13);
        fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
            void input;
            void init;
            return Promise.resolve(okResponse());
        });
        harness.windowListeners['pagehide']?.[0]?.(new Event('pagehide'));
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledTimes(23);
    });

    it('suppresses duplicate pageviews for the same path in a short window', () => {
        const harness = trackerHarness('/signup');

        bootstrapTracker(harness.win);
        harness.history.pushState({}, '', '/signup');

        expect(harness.sendBeacon).toHaveBeenCalledTimes(1);
    });

    it('does not bootstrap twice', () => {
        const harness = trackerHarness('/signup');

        bootstrapTracker(harness.win);
        bootstrapTracker(harness.win);

        expect(harness.sendBeacon).toHaveBeenCalledTimes(1);
        expect(harness.windowListeners['pagehide']?.length).toBe(1);
    });

    it('keeps attribution in memory and limits sessionStorage to the session tuple', () => {
        const harness = trackerHarness('/signup?utm_source=launch&utm_medium=email');

        bootstrapTracker(harness.win);
        harness.history.pushState({}, '', '/dashboard?utm_source=changed&utm_medium=social');

        const secondBeaconBody = harness.sendBeacon.mock.calls[1]?.[1] as unknown as Blob;

        expect(harness.sessionStorage.setItem.mock.calls.every(([key]) => key === 'hk_session')).toBe(true);
        expect(harness.sessionStorage.setItem.mock.calls[0]?.[1]).toMatch(/^10000000-0000-4000-8000-000000000001\|\d+$/);
        return secondBeaconBody.text().then((body) => {
            const payload = JSON.parse(body) as Record<string, unknown>;
            expect(payload['u_src']).toBe('launch');
            expect(payload['u_med']).toBe('email');
        });
    });
});
