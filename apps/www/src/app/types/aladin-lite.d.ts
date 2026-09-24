declare module 'aladin-lite' {
  export interface AladinInstance {
    pix2world(x: number, y: number, frame?: string): [number, number] | null | undefined;
  }

  export interface AladinOptions {
    survey?: string;
    fov?: number;
    cooFrame?: string;
  }

  export interface AladinStatic {
    init: Promise<void>;
    aladin(element: HTMLElement, options?: AladinOptions): AladinInstance;
  }

  const A: AladinStatic;
  export default A;
}
