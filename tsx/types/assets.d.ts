// What asset imports give: the compiler reads them (tsx/compiler).

declare module "*.png" {
  /** The image, sized in points (@2x files at half their pixels). */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.jpg" {
  /** The image, sized in points (@2x files at half their pixels). */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.jpeg" {
  /** The image, sized in points (@2x files at half their pixels). */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.gif" {
  /** The image, sized in points (@2x files at half their pixels). */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.webp" {
  /** The image, sized in points (@2x files at half their pixels). */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.svg" {
  /** The SVG image, sized in points by its width and height, or its viewBox. */
  const asset: import("./types").ImageAsset;
  export default asset;
}

declare module "*.ttf" {
  /** The font's family name, for fontFamily. */
  const family: string;
  export default family;
}

declare module "*.woff" {
  /** The font's family name, for fontFamily. */
  const family: string;
  export default family;
}

declare module "*.woff2" {
  /** The font's family name, for fontFamily. */
  const family: string;
  export default family;
}

declare module "*.otf" {
  /** The font's family name, for fontFamily. */
  const family: string;
  export default family;
}
