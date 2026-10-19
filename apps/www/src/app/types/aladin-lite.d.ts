declare module 'aladin-lite' {
  export interface AladinInstance {
    pix2world(
      x: number,
      y: number,
      frame?: string,
    ): [number, number] | null | undefined;
    getFov(): [number, number];
    getRaDec(): [number, number];
    gotoObject(
      target: string,
      callbacks?: {
        success?: (raDec: [number, number]) => void;
        error?: () => void;
      },
    ): void;
    on(event: 'zoomChanged' | 'positionChanged', cb: () => void): void;
  }

  export interface AladinOptions {
    survey?: string;
    fov?: number;
    cooFrame?: string;
    mode?: 'light' | 'dark';
    showReticle?: boolean;
    reticleColor?: string;
    reticleSize?: number;
  }

  export interface AladinStatic {
    init: Promise<void>;
    aladin(element: HTMLElement, options?: AladinOptions): AladinInstance;
  }

  const A: AladinStatic;
  export default A;
}
