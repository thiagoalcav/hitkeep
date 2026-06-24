import { ChangeDetectionStrategy, Component, ElementRef, computed, effect, input, viewChild } from '@angular/core';
import QRCodeStyling from 'qr-code-styling';
import { QRCodeStyle } from '@models/analytics.types';

type QRExportExtension = 'svg' | 'png';

@Component({
    selector: 'app-qr-code-preview',
    template: `<div #container class="qr-code-preview__canvas" [style.width.px]="size()" [style.height.px]="size()"></div>`,
    styleUrl: './qr-code-preview.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class QRCodePreview {
    data = input.required<string>();
    style = input<QRCodeStyle | null | undefined>(null);
    imageURL = input<string | null>(null);
    size = input(260);

    private readonly container = viewChild<ElementRef<HTMLElement>>('container');
    private qr: QRCodeStyling | null = null;
    private lastImageURL: string | null = null;

    private readonly options = computed(() => {
        return this.buildOptions(this.size(), this.imageURL() || undefined);
    });

    constructor() {
        effect(() => {
            const container = this.container()?.nativeElement;
            const options = this.options();
            if (!container) return;

            if (!this.qr || this.lastImageURL !== this.imageURL()) {
                container.replaceChildren();
                this.qr = new QRCodeStyling(options);
                this.qr.append(container);
                this.lastImageURL = this.imageURL();
                return;
            }

            this.qr.update(options);
        });
    }

    async export(filename: string, extension: QRExportExtension, size = this.size()): Promise<void> {
        const embeddedImageURL = await this.embeddedImageURL();
        const qr = new QRCodeStyling(this.buildOptions(size, embeddedImageURL));
        const blob = await qr.getRawData(extension);
        if (!blob || !(blob instanceof Blob)) return;
        this.downloadBlob(blob, filename);
    }

    private buildOptions(size: number, imageURL?: string) {
        const style = this.style() ?? {};
        const foreground = style.foreground || '#111827';
        const background = style.background || '#ffffff';
        const dotType = style.dots || 'rounded';
        const cornerType = style.corners || 'extra-rounded';

        return {
            type: 'svg' as const,
            width: size,
            height: size,
            margin: Math.max(10, Math.round(size * 0.04)),
            data: this.data() || 'https://hitkeep.com',
            image: imageURL || undefined,
            qrOptions: {
                errorCorrectionLevel: imageURL ? ('H' as const) : ('Q' as const)
            },
            dotsOptions: {
                type: dotType,
                color: foreground
            },
            cornersSquareOptions: {
                type: cornerType,
                color: foreground
            },
            cornersDotOptions: {
                type: style.corners === 'dot' ? ('dot' as const) : ('square' as const),
                color: foreground
            },
            backgroundOptions: {
                color: background
            },
            imageOptions: {
                hideBackgroundDots: true,
                imageSize: 0.24,
                margin: style.image_margin ?? 6
            }
        };
    }

    private async embeddedImageURL(): Promise<string | undefined> {
        const imageURL = this.imageURL();
        if (!imageURL) return undefined;
        if (imageURL.startsWith('data:') || imageURL.startsWith('blob:')) return imageURL;
        try {
            const response = await fetch(imageURL, { credentials: 'same-origin' });
            if (!response.ok) return imageURL;
            const blob = await response.blob();
            return await new Promise<string>((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = () => resolve(String(reader.result || imageURL));
                reader.onerror = () => reject(reader.error);
                reader.readAsDataURL(blob);
            });
        } catch {
            return imageURL;
        }
    }

    private downloadBlob(blob: Blob, filename: string): void {
        const objectURL = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = objectURL;
        link.download = filename;
        link.style.display = 'none';
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(objectURL);
    }
}
