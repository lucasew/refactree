export enum Color {
  Helper = 1,
  Stay = 2,
}
export function use(): number {
  return Color.Helper + Color.Stay;
}
