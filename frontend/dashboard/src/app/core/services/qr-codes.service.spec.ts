import { buildQRCodeDestination, qrExportFilename } from './qr-codes.service';

describe('QR code helpers', () => {
    it('builds a tracked destination URL with UTM, custom parameters, and QR attribution', () => {
        const url = buildQRCodeDestination(
            {
                destination_url: 'https://example.com/landing?existing=1',
                utm_source: 'poster',
                utm_medium: 'print',
                utm_campaign: 'spring launch',
                utm_term: '',
                utm_content: 'front-window',
                custom_params: {
                    region: 'berlin',
                    empty: ''
                }
            },
            '4f7d9e6a-2b6f-4e70-a0f5-cc5d6e5f3a20'
        );

        expect(url).toBe('https://example.com/landing?existing=1&utm_source=poster&utm_medium=print&utm_campaign=spring+launch&utm_content=front-window&region=berlin&hk_qr=4f7d9e6a-2b6f-4e70-a0f5-cc5d6e5f3a20');
    });

    it('creates deterministic print export filenames', () => {
        expect(qrExportFilename('acme.example.com', 'Spring Launch / Berlin', 'print-2048', 'png')).toBe('acme-example-com-spring-launch-berlin-print-2048.png');
    });
});
