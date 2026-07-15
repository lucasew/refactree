export enum Color {
  Helper,
  Stay,
}
export function use(): number {
  return Color.Helper + Color.Stay;
}
